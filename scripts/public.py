import argparse
import os
import signal
import subprocess
import sys
import time
from collections import deque
from pathlib import Path
from typing import Deque, Optional, TextIO
from urllib.parse import urlparse

import local
from launcher_common import (
    LauncherConfig,
    LauncherError,
    load_launcher_config,
    poll_http_until_healthy,
    print_actionable_error,
    require_cloudflared,
    run_browser_smoke_check,
    run_command,
    validate_cfargotunnel_cname,
)


REPO_ROOT = Path(__file__).resolve().parents[1]
LEGACY_PUBLIC_TUNNEL_CONTAINER_NAME = "hermestoken-public-cloudflared"
PUBLIC_TUNNEL_RUNTIME_DIR_NAME = ".runtime"
PUBLIC_TUNNEL_PID_FILE_NAME = "public-cloudflared.pid"
PUBLIC_TUNNEL_LOG_FILE_NAME = "public-cloudflared.log"
TUNNEL_HEARTBEAT_SECONDS = 0.2
PUBLIC_PROBE_TIMEOUT_SECONDS = 5.0
PUBLIC_PROBE_INTERVAL_SECONDS = 0.2
RECENT_TUNNEL_LOG_LINES = 20
PROCESS_SHUTDOWN_TIMEOUT_SECONDS = 5.0


def _resolve_cloudflared_config_path(config_path: str, *, repo_root: Path) -> Path:
    resolved = Path(config_path).expanduser()
    if resolved.is_absolute():
        return resolved
    return repo_root / resolved


def _resolve_cloudflared_token_path(token_path: str, *, repo_root: Path) -> Path:
    resolved = Path(token_path).expanduser()
    if resolved.is_absolute():
        return resolved
    return repo_root / resolved


def _resolve_token_source(
    config: LauncherConfig,
    *,
    repo_root: Path,
) -> Optional[str]:
    if config.cloudflared_tunnel_token:
        return config.cloudflared_tunnel_token.strip()

    if not config.cloudflared_tunnel_token_path:
        return None

    token_path = _resolve_cloudflared_token_path(config.cloudflared_tunnel_token_path, repo_root=repo_root)
    if not token_path.is_file():
        raise LauncherError(
            f"cloudflared token file not found: {token_path} Next step: Create the token file or update `cloudflared_tunnel_token_path` in scripts/launcher_config.json."
        )

    token = token_path.read_text(encoding="utf-8").strip()
    if not token:
        raise LauncherError(
            f"cloudflared token file is empty: {token_path} Next step: Paste the tunnel token into the file and retry."
        )
    return token


def _extract_hostname_from_url(url: str) -> str:
    parsed = urlparse(url)
    hostname = parsed.hostname
    if not hostname:
        raise LauncherError(
            f"Invalid `public_url` value: {url} Next step: Set a valid https URL in scripts/launcher_config.json."
        )
    return hostname


def _tunnel_runtime_dir_path(*, repo_root: Path) -> Path:
    return repo_root / "scripts" / PUBLIC_TUNNEL_RUNTIME_DIR_NAME


def _tunnel_pid_file_path(*, repo_root: Path) -> Path:
    return _tunnel_runtime_dir_path(repo_root=repo_root) / PUBLIC_TUNNEL_PID_FILE_NAME


def _tunnel_log_file_path(*, repo_root: Path) -> Path:
    return _tunnel_runtime_dir_path(repo_root=repo_root) / PUBLIC_TUNNEL_LOG_FILE_NAME


def _ensure_tunnel_runtime_dir(*, repo_root: Path) -> Path:
    runtime_dir = _tunnel_runtime_dir_path(repo_root=repo_root)
    runtime_dir.mkdir(parents=True, exist_ok=True)
    return runtime_dir


def _read_tunnel_pid(*, repo_root: Path) -> Optional[int]:
    pid_path = _tunnel_pid_file_path(repo_root=repo_root)
    if not pid_path.is_file():
        return None

    try:
        raw_pid = pid_path.read_text(encoding="utf-8").strip()
        return int(raw_pid)
    except (OSError, ValueError):
        pid_path.unlink(missing_ok=True)
        return None


def _write_tunnel_pid(pid: int, *, repo_root: Path) -> None:
    _ensure_tunnel_runtime_dir(repo_root=repo_root)
    _tunnel_pid_file_path(repo_root=repo_root).write_text(f"{pid}\n", encoding="utf-8")


def _clear_tunnel_pid(*, repo_root: Path) -> None:
    _tunnel_pid_file_path(repo_root=repo_root).unlink(missing_ok=True)


def _process_exists(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True


def _is_tracked_tunnel_process(pid: int, *, repo_root: Path) -> bool:
    inspection = run_command(
        ["ps", "-p", str(pid), "-o", "command="],
        check=False,
        stream_output=False,
        cwd=repo_root,
    )
    command = (inspection.stdout or "").strip().lower()
    return inspection.returncode == 0 and "cloudflared" in command


def _cleanup_legacy_tunnel_container(*, repo_root: Path) -> None:
    run_command(
        ["docker", "rm", "-f", LEGACY_PUBLIC_TUNNEL_CONTAINER_NAME],
        check=False,
        stream_output=False,
        cwd=repo_root,
    )


def _stop_tunnel_process(*, repo_root: Path) -> None:
    pid = _read_tunnel_pid(repo_root=repo_root)
    if pid is not None and _is_tracked_tunnel_process(pid, repo_root=repo_root):
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        else:
            deadline = time.monotonic() + PROCESS_SHUTDOWN_TIMEOUT_SECONDS
            while time.monotonic() < deadline and _process_exists(pid):
                time.sleep(0.1)
            if _process_exists(pid):
                try:
                    os.kill(pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass

    _clear_tunnel_pid(repo_root=repo_root)
    _cleanup_legacy_tunnel_container(repo_root=repo_root)


def _start_tunnel_process(
    config: LauncherConfig,
    *,
    cloudflared_token: Optional[str],
    cloudflared_config_path: Optional[Path],
    repo_root: Path,
) -> int:
    cloudflared_binary = require_cloudflared()
    _ensure_tunnel_runtime_dir(repo_root=repo_root)
    log_path = _tunnel_log_file_path(repo_root=repo_root)
    _stop_tunnel_process(repo_root=repo_root)

    command = [cloudflared_binary, "tunnel", "--no-autoupdate"]
    environment = os.environ.copy()

    if cloudflared_token is not None:
        environment["TUNNEL_TOKEN"] = cloudflared_token
        command.append("run")
    else:
        if cloudflared_config_path is None:
            raise LauncherError(
                "Missing cloudflared configuration. Next step: Set a tunnel token or a valid `cloudflared_config_path` in scripts/launcher_config.json."
            )
        command.extend(["--config", str(cloudflared_config_path), "run", config.cloudflared_tunnel_name])

    with log_path.open("w", encoding="utf-8") as log_stream:
        process = subprocess.Popen(
            command,
            cwd=str(repo_root),
            env=environment,
            stdout=log_stream,
            stderr=subprocess.STDOUT,
            text=True,
            start_new_session=True,
        )

    _write_tunnel_pid(process.pid, repo_root=repo_root)
    time.sleep(TUNNEL_HEARTBEAT_SECONDS)
    exit_code = process.poll()
    if exit_code is not None:
        _clear_tunnel_pid(repo_root=repo_root)
        raise LauncherError(
            f"cloudflared exited immediately with code {exit_code}. Next step: Review the cloudflared logs and tunnel configuration, then rerun scripts/public.py."
            + _format_tunnel_output_context(_read_recent_tunnel_output(repo_root=repo_root))
        )

    return process.pid


def _read_recent_tunnel_output(*, repo_root: Path, max_lines: int = RECENT_TUNNEL_LOG_LINES) -> Deque[str]:
    lines: Deque[str] = deque(maxlen=max_lines)
    log_path = _tunnel_log_file_path(repo_root=repo_root)
    if not log_path.is_file():
        return lines

    try:
        for line in log_path.read_text(encoding="utf-8").splitlines()[-max_lines:]:
            text = line.strip()
            if text:
                lines.append(text)
    except OSError:
        return lines
    return lines


def _format_tunnel_output_context(log_buffer: Deque[str]) -> str:
    if not log_buffer:
        return ""
    return " Recent cloudflared output: " + " | ".join(log_buffer)


def _current_tunnel_process_status(*, repo_root: Path) -> str:
    pid = _read_tunnel_pid(repo_root=repo_root)
    if pid is None:
        return ""
    if not _process_exists(pid):
        _clear_tunnel_pid(repo_root=repo_root)
        return "exited"
    if not _is_tracked_tunnel_process(pid, repo_root=repo_root):
        _clear_tunnel_pid(repo_root=repo_root)
        return ""
    return "running"


def _wait_for_public_readiness(
    config: LauncherConfig,
    *,
    repo_root: Path,
) -> None:
    deadline = time.monotonic() + config.healthcheck_timeout_seconds
    last_health_error: Optional[str] = None

    while True:
        process_status = _current_tunnel_process_status(repo_root=repo_root)
        if process_status and process_status != "running":
            raise LauncherError(
                f"Tunnel process entered `{process_status}` before {config.public_url} became healthy. "
                "Next step: Check the cloudflared logs and tunnel configuration, then rerun scripts/public.py."
                + _format_tunnel_output_context(_read_recent_tunnel_output(repo_root=repo_root))
            )

        remaining_seconds = deadline - time.monotonic()
        if remaining_seconds <= 0:
            details = f" Last health error: {last_health_error}" if last_health_error else ""
            raise LauncherError(
                f"Health check timed out for {config.public_url}. Next step: Service never became healthy within "
                f"{config.healthcheck_timeout_seconds}s.{details}"
                + _format_tunnel_output_context(_read_recent_tunnel_output(repo_root=repo_root))
            )

        probe_timeout_seconds = min(PUBLIC_PROBE_TIMEOUT_SECONDS, remaining_seconds)
        probe_interval_seconds = min(PUBLIC_PROBE_INTERVAL_SECONDS, probe_timeout_seconds)

        try:
            poll_http_until_healthy(
                config.public_url,
                timeout_seconds=probe_timeout_seconds,
                interval_seconds=probe_interval_seconds,
            )
            return
        except LauncherError as exc:
            last_health_error = str(exc)
            sleep_for = min(TUNNEL_HEARTBEAT_SECONDS, max(0.0, deadline - time.monotonic()))
            if sleep_for > 0:
                time.sleep(sleep_for)


def run_public_stack(
    config: LauncherConfig,
    *,
    output: Optional[TextIO] = None,
    repo_root: Optional[Path] = None,
    action_label: str = "deploy",
) -> None:
    stream = output or sys.stdout
    effective_repo_root = repo_root or REPO_ROOT

    cloudflared_token = _resolve_token_source(config, repo_root=effective_repo_root)
    using_token_auth = bool(cloudflared_token)
    cloudflared_config_path = None
    if not using_token_auth:
        cloudflared_config_path = _resolve_cloudflared_config_path(config.cloudflared_config_path, repo_root=effective_repo_root)
        if not cloudflared_config_path.is_file():
            raise LauncherError(
                f"cloudflared config file not found: {cloudflared_config_path} Next step: Set `cloudflared_config_path` in scripts/launcher_config.json to an existing config file."
            )
    else:
        stream.write("[ok] cloudflared token auth configured\n")

    public_hostname = _extract_hostname_from_url(config.public_url)
    if using_token_auth:
        stream.write(f"[ok] Public hostname configured in Cloudflare: {public_hostname}\n")
    else:
        validate_cfargotunnel_cname(public_hostname)
        stream.write(f"[ok] DNS configured for {public_hostname}\n")

    local.run_local_stack(config, output=stream, repo_root=effective_repo_root, action_label=action_label)

    try:
        tunnel_pid = _start_tunnel_process(
            config,
            cloudflared_token=cloudflared_token,
            cloudflared_config_path=cloudflared_config_path,
            repo_root=effective_repo_root,
        )
        stream.write(f"[ok] Tunnel process started: {tunnel_pid}\n")
        _wait_for_public_readiness(config, repo_root=effective_repo_root)
        stream.write(f"[ok] Public {action_label} healthy: {config.public_url}\n")
        stream.write(f"[ok] Local URL:  {config.local_url}\n")
        stream.write(f"[ok] Public URL: {config.public_url}\n")
    except Exception as exc:
        log_context = _format_tunnel_output_context(_read_recent_tunnel_output(repo_root=effective_repo_root))
        _stop_tunnel_process(repo_root=effective_repo_root)
        if isinstance(exc, LauncherError):
            if log_context and log_context not in str(exc):
                raise LauncherError(f"{exc}{log_context}") from exc
        raise

    try:
        run_browser_smoke_check(config.public_url, output=stream)
    except LauncherError as exc:
        stream.write(f"[warn] Public browser smoke check failed after health check: {exc}\n")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Deploy or update the local-public Docker stack for HERMESTOKEN.")
    parser.add_argument("command", nargs="?", choices=("deploy", "update"), default="deploy")
    return parser


def main() -> int:
    try:
        args = build_parser().parse_args()
        config = load_launcher_config()
        run_public_stack(config, output=sys.stdout, action_label=args.command)
        return 0
    except LauncherError as exc:
        print_actionable_error(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
