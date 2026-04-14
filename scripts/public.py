import subprocess
import sys
import tempfile
import time
from collections import deque
from pathlib import Path
from typing import Deque, Optional, TextIO, Tuple
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
    validate_cfargotunnel_cname,
)


REPO_ROOT = Path(__file__).resolve().parents[1]
TUNNEL_HEARTBEAT_SECONDS = 0.2
PUBLIC_PROBE_TIMEOUT_SECONDS = 5.0
PUBLIC_PROBE_INTERVAL_SECONDS = 0.2
RECENT_TUNNEL_LOG_LINES = 20


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


def _load_cloudflared_token(config: LauncherConfig, *, repo_root: Path) -> Optional[str]:
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


def _stop_tunnel_process(process: subprocess.Popen) -> None:
    if process.poll() is not None:
        return

    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            return


def _start_tunnel_process(
    config: LauncherConfig,
    *,
    cloudflared_token: Optional[str],
    cloudflared_config_path: Path,
    repo_root: Path,
) -> Tuple[subprocess.Popen, Path]:
    if cloudflared_token:
        command = [
            "cloudflared",
            "tunnel",
            "run",
            "--token",
            cloudflared_token,
        ]
    else:
        command = [
            "cloudflared",
            "tunnel",
            "--config",
            str(cloudflared_config_path),
            "run",
            config.cloudflared_tunnel_name,
        ]
    log_file = tempfile.NamedTemporaryFile(
        mode="a",
        encoding="utf-8",
        prefix="cloudflared-launcher-",
        suffix=".log",
        delete=False,
    )
    log_path = Path(log_file.name)
    try:
        process = subprocess.Popen(
            command,
            cwd=str(repo_root),
            stdout=log_file,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )
        return process, log_path
    except OSError as exc:
        log_file.close()
        try:
            log_path.unlink(missing_ok=True)
        except OSError:
            pass
        raise LauncherError(
            f"Unable to start Cloudflare tunnel process. Next step: Verify cloudflared config and tunnel name, then retry. Details: {exc}"
        ) from exc
    finally:
        log_file.close()


def _read_recent_tunnel_output(log_path: Optional[Path], *, max_lines: int = RECENT_TUNNEL_LOG_LINES) -> Deque[str]:
    lines: Deque[str] = deque(maxlen=max_lines)
    if log_path is None:
        return lines

    try:
        with log_path.open("r", encoding="utf-8", errors="replace") as handle:
            for line in handle:
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


def _wait_for_public_readiness(
    config: LauncherConfig,
    tunnel_process: subprocess.Popen,
    *,
    log_buffer: Optional[Deque[str]] = None,
) -> None:
    deadline = time.monotonic() + config.healthcheck_timeout_seconds
    tunnel_logs = log_buffer or deque(maxlen=RECENT_TUNNEL_LOG_LINES)
    last_health_error: Optional[str] = None

    while True:
        exit_code = tunnel_process.poll()
        if exit_code is not None:
            raise LauncherError(
                f"Tunnel process exited unexpectedly with exit code {exit_code} before {config.public_url} became healthy. "
                "Next step: Check cloudflared tunnel configuration and logs, then rerun scripts/public.py."
                + _format_tunnel_output_context(tunnel_logs)
            )

        remaining_seconds = deadline - time.monotonic()
        if remaining_seconds <= 0:
            details = f" Last health error: {last_health_error}" if last_health_error else ""
            raise LauncherError(
                f"Health check timed out for {config.public_url}. Next step: Service never became healthy within "
                f"{config.healthcheck_timeout_seconds}s.{details}"
                + _format_tunnel_output_context(tunnel_logs)
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


def run_public_stack(config: LauncherConfig, *, output: Optional[TextIO] = None, repo_root: Optional[Path] = None) -> None:
    stream = output or sys.stdout
    effective_repo_root = repo_root or REPO_ROOT

    require_cloudflared()
    stream.write("[ok] cloudflared available\n")

    cloudflared_config_path = _resolve_cloudflared_config_path(config.cloudflared_config_path, repo_root=effective_repo_root)
    cloudflared_token = _load_cloudflared_token(config, repo_root=effective_repo_root)
    using_token_auth = bool(cloudflared_token)
    if not using_token_auth and not cloudflared_config_path.is_file():
        raise LauncherError(
            f"cloudflared config file not found: {cloudflared_config_path} Next step: Set `cloudflared_config_path` in scripts/launcher_config.json to an existing config file."
        )
    if using_token_auth:
        stream.write("[ok] cloudflared token auth configured\n")

    public_hostname = _extract_hostname_from_url(config.public_url)
    if using_token_auth:
        stream.write(f"[ok] Public hostname configured in Cloudflare: {public_hostname}\n")
    else:
        validate_cfargotunnel_cname(public_hostname)
        stream.write(f"[ok] DNS configured for {public_hostname}\n")

    # Reuse the local launcher so public startup inherits the default rebuild behavior.
    local.run_local_stack(config, output=stream, repo_root=effective_repo_root)

    tunnel_process: Optional[subprocess.Popen] = None
    tunnel_log_path: Optional[Path] = None
    try:
        tunnel_process, tunnel_log_path = _start_tunnel_process(
            config,
            cloudflared_token=cloudflared_token,
            cloudflared_config_path=cloudflared_config_path,
            repo_root=effective_repo_root,
        )
        stream.write(f"[ok] Tunnel started: {config.cloudflared_tunnel_name}\n")

        _wait_for_public_readiness(config, tunnel_process)
        run_browser_smoke_check(config.public_url, output=stream)
        stream.write(f"[ok] Public service healthy: {config.public_url}\n")
        stream.write(f"[ok] Local URL:  {config.local_url}\n")
        stream.write(f"[ok] Public URL: {config.public_url}\n")
    except Exception as exc:
        if tunnel_process is not None:
            _stop_tunnel_process(tunnel_process)
        if isinstance(exc, LauncherError):
            log_context = _format_tunnel_output_context(_read_recent_tunnel_output(tunnel_log_path))
            if log_context and log_context not in str(exc):
                raise LauncherError(f"{exc}{log_context}") from exc
        raise
    finally:
        if tunnel_process is not None and tunnel_process.poll() is None:
            # Success path intentionally leaves tunnel running in background.
            pass
        elif tunnel_log_path is not None:
            try:
                tunnel_log_path.unlink(missing_ok=True)
            except OSError:
                pass


def main() -> int:
    try:
        config = load_launcher_config()
        run_public_stack(config, output=sys.stdout)
        return 0
    except LauncherError as exc:
        print_actionable_error(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
