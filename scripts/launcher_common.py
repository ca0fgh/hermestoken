import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable, Mapping, Optional, Sequence, TextIO, Tuple, Union
from urllib.error import HTTPError, URLError


class LauncherError(RuntimeError):
    """Raised when launcher preconditions or orchestration steps fail."""


DEFAULT_HEALTHCHECK_USER_AGENT = "hermestoken-launcher/1.0 (+https://pay-local.hermestoken.top)"


@dataclass(frozen=True)
class LauncherConfig:
    compose_file: str
    local_url: str
    public_url: str
    cloudflared_tunnel_name: str
    cloudflared_config_path: str
    healthcheck_timeout_seconds: float
    healthcheck_interval_seconds: float
    cloudflared_tunnel_token: Optional[str] = None
    cloudflared_tunnel_token_path: Optional[str] = None


def _build_actionable_message(problem: str, action: Optional[str] = None) -> str:
    if action:
        return f"{problem} Next step: {action}"
    return problem


def print_actionable_error(message: str, action: Optional[str] = None, stream: Optional[TextIO] = None) -> None:
    output = stream or sys.stderr
    output.write(f"[error] {_build_actionable_message(message, action)}\n")


def _validate_config_field(
    data: Mapping[str, Any],
    key: str,
    expected_type: Union[type, Tuple[type, ...]],
) -> Any:
    if key not in data:
        raise LauncherError(
            _build_actionable_message(
                f"Missing required config field: {key}",
                "Add the missing field to scripts/launcher_config.json.",
            )
        )
    value = data[key]
    if isinstance(value, bool):
        bool_allowed = (
            (isinstance(expected_type, tuple) and bool in expected_type)
            or (not isinstance(expected_type, tuple) and expected_type is bool)
        )
        if not bool_allowed:
            if isinstance(expected_type, tuple):
                expected_names = ", ".join(t.__name__ for t in expected_type)
            else:
                expected_names = expected_type.__name__
            raise LauncherError(
                _build_actionable_message(
                    f"Invalid config field `{key}` type: expected {expected_names}, got bool.",
                    "Use numeric values (not true/false) in scripts/launcher_config.json.",
                )
            )
    if not isinstance(value, expected_type):
        if isinstance(expected_type, tuple):
            expected_names = ", ".join(t.__name__ for t in expected_type)
        else:
            expected_names = expected_type.__name__
        raise LauncherError(
            _build_actionable_message(
                f"Invalid config field `{key}` type: expected {expected_names}.",
                "Fix the field type in scripts/launcher_config.json.",
            )
        )
    if isinstance(value, str) and not value.strip():
        raise LauncherError(
            _build_actionable_message(
                f"Invalid config field `{key}`: value cannot be empty.",
                "Provide a non-empty value in scripts/launcher_config.json.",
            )
        )
    return value


def _validate_optional_string_field(data: Mapping[str, Any], key: str) -> Optional[str]:
    if key not in data:
        return None

    value = data[key]
    if value is None:
        return None
    if not isinstance(value, str):
        raise LauncherError(
            _build_actionable_message(
                f"Invalid config field `{key}` type: expected str.",
                "Fix the field type in scripts/launcher_config.json.",
            )
        )

    trimmed = value.strip()
    if not trimmed:
        return None
    return trimmed


def load_launcher_config(config_path: Optional[Path] = None) -> LauncherConfig:
    path = config_path or Path(__file__).with_name("launcher_config.json")
    try:
        raw = path.read_text(encoding="utf-8")
    except FileNotFoundError as exc:
        raise LauncherError(
            _build_actionable_message(
                f"Launcher config not found: {path}",
                "Create scripts/launcher_config.json from the documented contract.",
            )
        ) from exc

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise LauncherError(
            _build_actionable_message(
                f"Invalid JSON in launcher config: {path}",
                "Fix JSON syntax in scripts/launcher_config.json.",
            )
        ) from exc

    if not isinstance(data, dict):
        raise LauncherError(
            _build_actionable_message(
                f"Launcher config must be a JSON object: {path}",
                "Wrap launcher settings in a top-level JSON object.",
            )
        )

    compose_file = _validate_config_field(data, "compose_file", str)
    local_url = _validate_config_field(data, "local_url", str)
    public_url = _validate_config_field(data, "public_url", str)
    cloudflared_tunnel_name = _validate_config_field(data, "cloudflared_tunnel_name", str)
    cloudflared_config_path = _validate_config_field(data, "cloudflared_config_path", str)
    cloudflared_tunnel_token = _validate_optional_string_field(data, "cloudflared_tunnel_token")
    cloudflared_tunnel_token_path = _validate_optional_string_field(data, "cloudflared_tunnel_token_path")
    timeout_seconds = _validate_config_field(data, "healthcheck_timeout_seconds", (int, float))
    interval_seconds = _validate_config_field(data, "healthcheck_interval_seconds", (int, float))

    if timeout_seconds <= 0:
        raise LauncherError(
            _build_actionable_message(
                "Invalid config field `healthcheck_timeout_seconds`: must be > 0.",
                "Set a positive timeout value in scripts/launcher_config.json.",
            )
        )
    if interval_seconds < 0:
        raise LauncherError(
            _build_actionable_message(
                "Invalid config field `healthcheck_interval_seconds`: must be >= 0.",
                "Set a non-negative interval in scripts/launcher_config.json.",
            )
        )

    return LauncherConfig(
        compose_file=compose_file,
        local_url=local_url,
        public_url=public_url,
        cloudflared_tunnel_name=cloudflared_tunnel_name,
        cloudflared_config_path=cloudflared_config_path,
        cloudflared_tunnel_token=cloudflared_tunnel_token,
        cloudflared_tunnel_token_path=cloudflared_tunnel_token_path,
        healthcheck_timeout_seconds=float(timeout_seconds),
        healthcheck_interval_seconds=float(interval_seconds),
    )


def require_executable(binary_name: str, install_hint: Optional[str] = None) -> str:
    resolved = shutil.which(binary_name)
    if resolved:
        return resolved

    hint = install_hint or f"Install `{binary_name}` and ensure it is available on PATH."
    raise LauncherError(_build_actionable_message(f"Missing required executable: {binary_name}", hint))


def require_docker_and_compose() -> None:
    require_executable(
        "docker",
        install_hint="Install Docker Desktop (or Docker Engine) and confirm `docker` works.",
    )
    try:
        run_command(["docker", "compose", "version"], check=True, stream_output=False)
    except LauncherError as exc:
        raise LauncherError(
            _build_actionable_message(
                "`docker compose` is unavailable.",
                "Upgrade to Docker Compose v2 and verify `docker compose version` succeeds.",
            )
        ) from exc


def require_cloudflared() -> str:
    return require_executable(
        "cloudflared",
        install_hint="Install cloudflared and verify `cloudflared --version` succeeds.",
    )


def run_command(
    command: Sequence[str],
    *,
    check: bool = True,
    stream_output: bool = False,
    cwd: Optional[Path] = None,
    env: Optional[Mapping[str, str]] = None,
    stdout_stream: Optional[TextIO] = None,
) -> subprocess.CompletedProcess:
    cmd_display = " ".join(command)
    merged_env = os.environ.copy()
    if env:
        merged_env.update(dict(env))

    if not stream_output:
        try:
            completed = subprocess.run(
                list(command),
                cwd=str(cwd) if cwd else None,
                env=merged_env,
                capture_output=True,
                text=True,
                check=False,
            )
        except FileNotFoundError as exc:
            raise LauncherError(
                _build_actionable_message(
                    f"Failed to run command: {cmd_display}",
                    "Verify the executable exists and is available on PATH.",
                )
            ) from exc

        if check and completed.returncode != 0:
            stderr = (completed.stderr or "").strip()
            raise LauncherError(
                _build_actionable_message(
                    f"Command failed ({completed.returncode}): {cmd_display}",
                    stderr or "Check command output and fix the failing dependency.",
                )
            )
        return completed

    stream = stdout_stream or sys.stdout
    output_lines = []

    try:
        process = subprocess.Popen(
            list(command),
            cwd=str(cwd) if cwd else None,
            env=merged_env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )
    except FileNotFoundError as exc:
        raise LauncherError(
            _build_actionable_message(
                f"Failed to run command: {cmd_display}",
                "Verify the executable exists and is available on PATH.",
            )
        ) from exc

    assert process.stdout is not None
    for line in process.stdout:
        output_lines.append(line)
        stream.write(line)

    return_code = process.wait()
    combined_output = "".join(output_lines)

    completed = subprocess.CompletedProcess(list(command), return_code, stdout=combined_output, stderr=None)
    if check and return_code != 0:
        output_tail = combined_output.strip()[-400:]
        details = f" Last output: {output_tail}" if output_tail else ""
        raise LauncherError(
            _build_actionable_message(
                f"Command failed ({return_code}): {cmd_display}.{details}",
                "Review the streamed logs above for the failing step.",
            )
        )
    return completed


def poll_http_until_healthy(
    url: str,
    *,
    timeout_seconds: float,
    interval_seconds: float,
    healthy_statuses: Iterable[int] = (200, 201, 202, 204, 301, 302),
) -> None:
    deadline = time.monotonic() + timeout_seconds
    healthy_set = set(healthy_statuses)
    last_error = ""

    while True:
        now = time.monotonic()
        remaining = deadline - now
        if remaining <= 0:
            break

        request = urllib.request.Request(
            url=url,
            method="GET",
            headers={
                "User-Agent": DEFAULT_HEALTHCHECK_USER_AGENT,
                "Accept": "*/*",
            },
        )
        try:
            with urllib.request.urlopen(request, timeout=remaining) as response:
                status = getattr(response, "status", 200)
                if status in healthy_set:
                    return
                last_error = f"unexpected HTTP status: {status}"
        except HTTPError as exc:
            if exc.code in healthy_set:
                return
            last_error = f"unexpected HTTP status: {exc.code}"
        except URLError as exc:
            last_error = str(exc.reason or exc)
        except OSError as exc:
            # Container restarts can briefly reset local TCP connections
            # before the HTTP server is fully ready. Treat that as retriable.
            last_error = str(exc)

        sleep_for = min(interval_seconds, max(0.0, deadline - time.monotonic()))
        if sleep_for > 0:
            time.sleep(sleep_for)

    raise LauncherError(
        _build_actionable_message(
            f"Health check timed out for {url}.",
            f"Service never became healthy within {timeout_seconds}s. Last error: {last_error or 'no response'}",
        )
    )


def _extract_cname_target(nslookup_output: str) -> Optional[str]:
    patterns = [
        r"canonical\s+name\s*=\s*([^\s]+)",
        r"is\s+an\s+alias\s+for\s+([^\s]+)",
        r"cname\s*=\s*([^\s]+)",
    ]
    for pattern in patterns:
        match = re.search(pattern, nslookup_output or "", flags=re.IGNORECASE)
        if match:
            return match.group(1)
    return None


def validate_cfargotunnel_cname(hostname: str, *, lookup_timeout_seconds: float = 5.0) -> str:
    try:
        completed = subprocess.run(
            ["nslookup", "-type=CNAME", hostname],
            capture_output=True,
            text=True,
            check=False,
            timeout=lookup_timeout_seconds,
        )
    except FileNotFoundError as exc:
        raise LauncherError(
            _build_actionable_message(
                "Unable to validate DNS CNAME because `nslookup` is unavailable.",
                "Install DNS lookup tooling that provides `nslookup` and rerun the launcher.",
            )
        ) from exc
    except subprocess.TimeoutExpired as exc:
        raise LauncherError(
            _build_actionable_message(
                f"DNS lookup timed out for {hostname}.",
                f"Retry and verify DNS/network reachability (timeout={lookup_timeout_seconds}s).",
            )
        ) from exc

    if completed.returncode != 0:
        details = (completed.stderr or completed.stdout or "").strip()
        raise LauncherError(
            _build_actionable_message(
                f"Failed to resolve CNAME for {hostname}.",
                details or "Check local DNS/network settings and try again.",
            )
        )

    cname_target = _extract_cname_target(completed.stdout or "")
    if not cname_target:
        raise LauncherError(
            _build_actionable_message(
                f"No CNAME record found for {hostname}.",
                "Create a DNS CNAME that points to your tunnel-id.cfargotunnel.com target.",
            )
        )

    cname = cname_target.rstrip(".").lower()
    if not cname.endswith(".cfargotunnel.com"):
        raise LauncherError(
            _build_actionable_message(
                f"CNAME target for {hostname} is `{cname}`, expected a `.cfargotunnel.com` target.",
                "Update DNS CNAME to your Cloudflare tunnel endpoint and retry.",
            )
        )

    return cname
