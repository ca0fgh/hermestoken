import argparse
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
    run_browser_smoke_check,
    run_command,
    validate_cfargotunnel_cname,
)


REPO_ROOT = Path(__file__).resolve().parents[1]
LOCAL_APP_CONTAINER_NAME = "new-api"
PUBLIC_TUNNEL_CONTAINER_NAME = "hermestoken-public-cloudflared"
CLOUDFLARED_IMAGE = "cloudflare/cloudflared:latest"
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


def _extract_cloudflared_reference_path(config_path: Path, field_name: str) -> Optional[Path]:
    prefix = f"{field_name}:"
    try:
        for line in config_path.read_text(encoding="utf-8").splitlines():
            stripped = line.strip()
            if not stripped.startswith(prefix):
                continue
            raw_value = stripped[len(prefix) :].strip().strip("'\"")
            if not raw_value:
                return None
            resolved = Path(raw_value).expanduser()
            if not resolved.is_absolute():
                resolved = (config_path.parent / resolved).resolve()
            return resolved
    except OSError:
        return None
    return None


def _stop_tunnel_container(*, repo_root: Path) -> None:
    run_command(
        ["docker", "rm", "-f", PUBLIC_TUNNEL_CONTAINER_NAME],
        check=False,
        stream_output=False,
        cwd=repo_root,
    )


def _start_tunnel_container(
    config: LauncherConfig,
    *,
    cloudflared_token: Optional[str],
    cloudflared_config_path: Optional[Path],
    repo_root: Path,
) -> None:
    docker_command = [
        "docker",
        "run",
        "-d",
        "--name",
        PUBLIC_TUNNEL_CONTAINER_NAME,
        "--restart",
        "unless-stopped",
        "--network",
        f"container:{LOCAL_APP_CONTAINER_NAME}",
    ]

    if cloudflared_token is not None:
        docker_command.extend(
            [
                "-e",
                f"TUNNEL_TOKEN={cloudflared_token}",
                CLOUDFLARED_IMAGE,
                "tunnel",
                "--no-autoupdate",
                "run",
            ]
        )
    else:
        if cloudflared_config_path is None:
            raise LauncherError(
                "Missing cloudflared configuration. Next step: Set a tunnel token or a valid `cloudflared_config_path` in scripts/launcher_config.json."
            )

        mount_paths = {cloudflared_config_path.parent}
        credentials_path = _extract_cloudflared_reference_path(cloudflared_config_path, "credentials-file")
        if credentials_path is not None:
            mount_paths.add(credentials_path.parent)

        for mount_path in sorted(mount_paths):
            docker_command.extend(["-v", f"{mount_path}:{mount_path}:ro"])

        docker_command.extend(
            [
                CLOUDFLARED_IMAGE,
                "tunnel",
                "--no-autoupdate",
                "--config",
                str(cloudflared_config_path),
                "run",
                config.cloudflared_tunnel_name,
            ]
        )

    _stop_tunnel_container(repo_root=repo_root)
    run_command(
        docker_command,
        check=True,
        stream_output=False,
        cwd=repo_root,
    )


def _read_recent_tunnel_output(*, repo_root: Path, max_lines: int = RECENT_TUNNEL_LOG_LINES) -> Deque[str]:
    lines: Deque[str] = deque(maxlen=max_lines)
    completed = run_command(
        ["docker", "logs", "--tail", str(max_lines), PUBLIC_TUNNEL_CONTAINER_NAME],
        check=False,
        stream_output=False,
        cwd=repo_root,
    )
    for line in (completed.stdout or "").splitlines():
        text = line.strip()
        if text:
            lines.append(text)
    return lines


def _format_tunnel_output_context(log_buffer: Deque[str]) -> str:
    if not log_buffer:
        return ""
    return " Recent cloudflared output: " + " | ".join(log_buffer)


def _current_tunnel_container_status(*, repo_root: Path) -> str:
    completed = run_command(
        ["docker", "inspect", "-f", "{{.State.Status}}", PUBLIC_TUNNEL_CONTAINER_NAME],
        check=False,
        stream_output=False,
        cwd=repo_root,
    )
    return (completed.stdout or "").strip()


def _wait_for_public_readiness(
    config: LauncherConfig,
    *,
    repo_root: Path,
) -> None:
    deadline = time.monotonic() + config.healthcheck_timeout_seconds
    last_health_error: Optional[str] = None

    while True:
        container_status = _current_tunnel_container_status(repo_root=repo_root)
        if container_status and container_status != "running":
            raise LauncherError(
                f"Tunnel container entered `{container_status}` before {config.public_url} became healthy. "
                "Next step: Check the cloudflared container logs and tunnel configuration, then rerun scripts/public.py."
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
        _start_tunnel_container(
            config,
            cloudflared_token=cloudflared_token,
            cloudflared_config_path=cloudflared_config_path,
            repo_root=effective_repo_root,
        )
        stream.write(f"[ok] Tunnel container started: {PUBLIC_TUNNEL_CONTAINER_NAME}\n")
        _wait_for_public_readiness(config, repo_root=effective_repo_root)
        run_browser_smoke_check(config.public_url, output=stream)
        stream.write(f"[ok] Public {action_label} healthy: {config.public_url}\n")
        stream.write(f"[ok] Local URL:  {config.local_url}\n")
        stream.write(f"[ok] Public URL: {config.public_url}\n")
    except Exception as exc:
        log_context = _format_tunnel_output_context(_read_recent_tunnel_output(repo_root=effective_repo_root))
        _stop_tunnel_container(repo_root=effective_repo_root)
        if isinstance(exc, LauncherError):
            if log_context and log_context not in str(exc):
                raise LauncherError(f"{exc}{log_context}") from exc
        raise


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
