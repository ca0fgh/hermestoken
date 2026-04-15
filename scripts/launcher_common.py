import base64
import hashlib
import json
import os
import re
import shutil
import socket
import ssl
import subprocess
import sys
import tempfile
import time
import urllib.parse
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable, Mapping, Optional, Sequence, TextIO, Tuple, Union
from urllib.error import HTTPError, URLError


class LauncherError(RuntimeError):
    """Raised when launcher preconditions or orchestration steps fail."""


DEFAULT_HEALTHCHECK_USER_AGENT = "hermestoken-launcher/1.0 (+https://pay-local.hermestoken.top)"
DEFAULT_BROWSER_SETTLE_WINDOW_SECONDS = 0.5
DEFAULT_BROWSER_CANDIDATES = (
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
    "google-chrome",
    "chromium",
    "chromium-browser",
    "chrome",
)
DEFAULT_CA_BUNDLE_CANDIDATES = (
    "/etc/ssl/cert.pem",
    "/etc/ssl/certs/ca-certificates.crt",
    "/etc/pki/tls/certs/ca-bundle.crt",
    "/etc/ssl/ca-bundle.pem",
)
DEFAULT_WEB_DIST_STRATEGY = "prebuilt"
DEFAULT_WEB_BUILD_NODE_OPTIONS = "--max-old-space-size=4096"


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


@dataclass(frozen=True)
class BrowserSmokeCheckResult:
    status: str
    detail: str = ""


@dataclass(frozen=True)
class _DevToolsTarget:
    port: int
    websocket_url: str


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


def find_browser_executable(candidates: Sequence[str] = DEFAULT_BROWSER_CANDIDATES) -> Optional[str]:
    for candidate in candidates:
        expanded = os.path.expanduser(candidate)
        if os.path.isabs(expanded) and os.path.isfile(expanded) and os.access(expanded, os.X_OK):
            return expanded

        resolved = shutil.which(candidate)
        if resolved:
            return resolved
    return None


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


def resolve_web_dist_strategy(*, env: Optional[Mapping[str, str]] = None) -> str:
    candidate_env = env or os.environ
    raw_strategy = (candidate_env.get("WEB_DIST_STRATEGY") or "").strip().lower()
    if not raw_strategy:
        return DEFAULT_WEB_DIST_STRATEGY
    if raw_strategy != DEFAULT_WEB_DIST_STRATEGY:
        raise LauncherError(
            _build_actionable_message(
                f"Unsupported WEB_DIST_STRATEGY for launcher scripts: {raw_strategy}.",
                "Unset WEB_DIST_STRATEGY or set it to `prebuilt`. The launcher scripts forbid Docker-side frontend builds to avoid Docker-side Vite OOM.",
            )
        )
    return raw_strategy


def prepare_frontend_dist_for_docker_packaging(
    *,
    output: TextIO,
    repo_root: Path,
    env: Optional[Mapping[str, str]] = None,
) -> None:
    strategy = resolve_web_dist_strategy(env=env)
    candidate_env = env or os.environ

    require_executable(
        "bun",
        install_hint="Install Bun and verify `bun --version` succeeds. These launcher scripts build the frontend on the host so Docker only packages `web/dist` and avoids Docker-side Vite OOM.",
    )

    version_file = repo_root / "VERSION"
    if not version_file.is_file():
        raise LauncherError(
            _build_actionable_message(
                f"Missing VERSION file: {version_file}",
                "Restore the VERSION file and retry.",
            )
        )

    web_dir = repo_root / "web"
    if not web_dir.is_dir():
        raise LauncherError(
            _build_actionable_message(
                f"Missing frontend directory: {web_dir}",
                "Run the launcher from the repository root.",
            )
        )

    version = version_file.read_text(encoding="utf-8").strip()
    if not version:
        raise LauncherError(
            _build_actionable_message(
                f"VERSION file is empty: {version_file}",
                "Restore the application version before retrying.",
            )
        )

    output.write(f"[info] Building frontend on host before docker packaging (WEB_DIST_STRATEGY={strategy})...\n")
    run_command(
        ["bun", "install"],
        check=True,
        stream_output=True,
        cwd=web_dir,
        stdout_stream=output,
    )

    build_env = {
        "DISABLE_ESLINT_PLUGIN": "true",
        "VITE_REACT_APP_VERSION": version,
    }
    if "NODE_OPTIONS" not in candidate_env:
        build_env["NODE_OPTIONS"] = DEFAULT_WEB_BUILD_NODE_OPTIONS

    run_command(
        ["bun", "run", "build"],
        check=True,
        stream_output=True,
        cwd=web_dir,
        env=build_env,
        stdout_stream=output,
    )
    output.write("[ok] Host frontend build ready\n")


def remove_legacy_compose_containers(
    *,
    legacy_project_name: str,
    compose_file_path: Path,
    container_names: Sequence[str],
    output: Optional[TextIO] = None,
    repo_root: Optional[Path] = None,
) -> list[str]:
    stream = output or sys.stdout
    effective_repo_root = repo_root or compose_file_path.parent
    removed: list[str] = []
    inspect_format = (
        '{{ index .Config.Labels "com.docker.compose.project" }}'
        '|{{ index .Config.Labels "com.docker.compose.project.config_files" }}'
    )

    for container_name in container_names:
        inspection = run_command(
            ["docker", "inspect", "-f", inspect_format, container_name],
            check=False,
            stream_output=False,
            cwd=effective_repo_root,
        )
        if inspection.returncode != 0:
            continue

        project_name, _, config_files = (inspection.stdout or "").strip().partition("|")
        compose_paths = {part.strip() for part in config_files.split(",") if part.strip()}
        if project_name != legacy_project_name or str(compose_file_path) not in compose_paths:
            continue

        run_command(
            ["docker", "rm", "-f", container_name],
            check=True,
            stream_output=False,
            cwd=effective_repo_root,
        )
        removed.append(container_name)

    if removed:
        stream.write(f"[info] Removed legacy compose containers: {', '.join(removed)}\n")

    return removed


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
            with _urlopen_with_tls_fallback(request, timeout=remaining) as response:
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


def _is_https_url(url: str) -> bool:
    return urllib.parse.urlparse(url).scheme.lower() == "https"


def _is_certificate_verify_failure(error: URLError) -> bool:
    reason = getattr(error, "reason", error)
    message = str(reason or error).lower()
    return isinstance(reason, ssl.SSLCertVerificationError) or "certificate verify failed" in message


def _detect_fallback_ca_bundle_path() -> Optional[str]:
    candidates = []
    env_ca_bundle = os.environ.get("SSL_CERT_FILE")
    if env_ca_bundle:
        candidates.append(env_ca_bundle)
    candidates.extend(DEFAULT_CA_BUNDLE_CANDIDATES)

    try:
        import certifi  # type: ignore
    except ImportError:
        certifi = None

    if certifi is not None:
        candidates.append(certifi.where())

    seen = set()
    for candidate in candidates:
        if not candidate:
            continue
        normalized = os.path.expanduser(candidate)
        if normalized in seen:
            continue
        seen.add(normalized)
        if os.path.isfile(normalized):
            return normalized
    return None


def _urlopen_with_tls_fallback(request: urllib.request.Request, *, timeout: float) -> Any:
    try:
        return urllib.request.urlopen(request, timeout=timeout)
    except URLError as exc:
        if not _is_https_url(request.full_url) or not _is_certificate_verify_failure(exc):
            raise

        fallback_ca_bundle = _detect_fallback_ca_bundle_path()
        if not fallback_ca_bundle:
            raise

        fallback_context = ssl.create_default_context(cafile=fallback_ca_bundle)
        return urllib.request.urlopen(request, timeout=timeout, context=fallback_context)


def _extract_exception_detail(message: Mapping[str, Any]) -> str:
    params = message.get("params")
    if not isinstance(params, Mapping):
        return ""

    details = params.get("exceptionDetails")
    if not isinstance(details, Mapping):
        return ""

    exception = details.get("exception")
    if isinstance(exception, Mapping):
        description = exception.get("description")
        if isinstance(description, str) and description.strip():
            return description.strip()

    text = details.get("text")
    if isinstance(text, str) and text.strip():
        return text.strip()

    return ""


def _extract_target_from_devtools_list(payload: Any, *, port: int, target_type: Optional[str] = None) -> Optional[_DevToolsTarget]:
    if not isinstance(payload, list):
        return None

    for entry in payload:
        if not isinstance(entry, Mapping):
            continue
        if target_type is not None and entry.get("type") != target_type:
            continue
        websocket_url = entry.get("webSocketDebuggerUrl")
        if isinstance(websocket_url, str) and websocket_url.strip():
            return _DevToolsTarget(port=port, websocket_url=websocket_url)
    return None


def _read_json_url(url: str, *, timeout_seconds: float) -> Any:
    request = urllib.request.Request(url=url, method="GET", headers={"Accept": "application/json"})
    with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
        return json.loads(response.read().decode("utf-8"))


def _wait_for_devtools_target(profile_dir: Path, *, chrome_process: subprocess.Popen, timeout_seconds: float) -> _DevToolsTarget:
    deadline = time.monotonic() + timeout_seconds
    last_error = "browser debugging endpoint did not become ready"
    devtools_port_file = profile_dir / "DevToolsActivePort"

    while True:
        exit_code = chrome_process.poll()
        if exit_code is not None:
            raise LauncherError(f"browser exited before the debugging endpoint became ready (exit code {exit_code})")

        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise LauncherError(last_error)

        try:
            if devtools_port_file.is_file():
                lines = [line.strip() for line in devtools_port_file.read_text(encoding="utf-8").splitlines() if line.strip()]
                if len(lines) >= 2:
                    port = int(lines[0])
                    websocket_path = lines[1]
                    websocket_url = websocket_path
                    if not websocket_url.startswith("ws://") and not websocket_url.startswith("wss://"):
                        websocket_url = f"ws://127.0.0.1:{port}{websocket_path}"
                    return _DevToolsTarget(port=port, websocket_url=websocket_url)
                last_error = "browser debugging endpoint file was incomplete"
        except (OSError, ValueError) as exc:
            last_error = f"browser debugging endpoint not ready: {exc}"

        time.sleep(min(0.1, max(0.0, deadline - time.monotonic())))


def _create_page_target(port: int, *, timeout_seconds: float) -> Optional[_DevToolsTarget]:
    request = urllib.request.Request(
        url=f"http://127.0.0.1:{port}/json/new?about:blank",
        method="PUT",
        headers={"Accept": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
        payload = json.loads(response.read().decode("utf-8"))
    if isinstance(payload, Mapping):
        websocket_url = payload.get("webSocketDebuggerUrl")
        if isinstance(websocket_url, str) and websocket_url.strip():
            return _DevToolsTarget(port=port, websocket_url=websocket_url)
    return None


def _wait_for_page_target(port: int, *, chrome_process: subprocess.Popen, timeout_seconds: float) -> _DevToolsTarget:
    deadline = time.monotonic() + timeout_seconds
    last_error = "browser debugging endpoint did not expose a page target"

    while True:
        exit_code = chrome_process.poll()
        if exit_code is not None:
            raise LauncherError(f"browser exited before a page target became ready (exit code {exit_code})")

        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise LauncherError(last_error)

        try:
            payload = _read_json_url(
                f"http://127.0.0.1:{port}/json/list",
                timeout_seconds=min(1.0, remaining),
            )
            target = _extract_target_from_devtools_list(payload, port=port, target_type="page")
            if target is not None:
                return target

            target = _create_page_target(port, timeout_seconds=min(1.0, remaining))
            if target is not None:
                return target
            last_error = "browser debugging endpoint did not expose or create a page target"
        except (HTTPError, URLError, OSError, json.JSONDecodeError) as exc:
            last_error = f"browser debugging endpoint page target not ready: {exc}"

        time.sleep(min(0.1, max(0.0, deadline - time.monotonic())))


def _send_websocket_frame(sock: socket.socket, payload: str) -> None:
    _send_masked_websocket_frame(sock, opcode=0x1, payload=payload.encode("utf-8"))


def _send_masked_websocket_frame(sock: socket.socket, *, opcode: int, payload: bytes) -> None:
    header = bytearray([0x80 | (opcode & 0x0F)])
    payload_length = len(payload)
    if payload_length < 126:
        header.append(0x80 | payload_length)
    elif payload_length < (1 << 16):
        header.append(0x80 | 126)
        header.extend(payload_length.to_bytes(2, "big"))
    else:
        header.append(0x80 | 127)
        header.extend(payload_length.to_bytes(8, "big"))

    mask = os.urandom(4)
    header.extend(mask)
    masked_payload = bytes(byte ^ mask[index % 4] for index, byte in enumerate(payload))
    sock.sendall(bytes(header) + masked_payload)


def _recv_exact(sock: socket.socket, size: int) -> bytes:
    chunks = bytearray()
    while len(chunks) < size:
        chunk = sock.recv(size - len(chunks))
        if not chunk:
            raise LauncherError("browser debugging connection closed unexpectedly")
        chunks.extend(chunk)
    return bytes(chunks)


def _recv_websocket_frame(sock: socket.socket) -> str:
    while True:
        first, second = _recv_exact(sock, 2)
        opcode = first & 0x0F
        masked = bool(second & 0x80)
        payload_length = second & 0x7F

        if payload_length == 126:
            payload_length = int.from_bytes(_recv_exact(sock, 2), "big")
        elif payload_length == 127:
            payload_length = int.from_bytes(_recv_exact(sock, 8), "big")

        mask = _recv_exact(sock, 4) if masked else b""
        payload = _recv_exact(sock, payload_length) if payload_length else b""
        if masked:
            payload = bytes(byte ^ mask[index % 4] for index, byte in enumerate(payload))

        if opcode == 0x8:
            raise LauncherError("browser debugging connection closed unexpectedly")
        if opcode == 0x9:
            _send_masked_websocket_frame(sock, opcode=0xA, payload=payload)
            continue
        if opcode != 0x1:
            continue
        return payload.decode("utf-8")


def _connect_to_devtools(websocket_url: str, *, timeout_seconds: float) -> socket.socket:
    parsed = urllib.parse.urlparse(websocket_url)
    if parsed.scheme not in {"ws", "wss"}:
        raise LauncherError(f"Unsupported DevTools websocket URL: {websocket_url}")

    host = parsed.hostname or "127.0.0.1"
    port = parsed.port or (443 if parsed.scheme == "wss" else 80)
    path = parsed.path or "/"
    if parsed.query:
        path = f"{path}?{parsed.query}"

    sock = socket.create_connection((host, port), timeout=timeout_seconds)
    sock.settimeout(timeout_seconds)

    websocket_key = base64.b64encode(os.urandom(16)).decode("ascii")
    request = (
        f"GET {path} HTTP/1.1\r\n"
        f"Host: {host}:{port}\r\n"
        "Upgrade: websocket\r\n"
        "Connection: Upgrade\r\n"
        f"Sec-WebSocket-Key: {websocket_key}\r\n"
        "Sec-WebSocket-Version: 13\r\n"
        "\r\n"
    )
    sock.sendall(request.encode("ascii"))

    response = bytearray()
    while b"\r\n\r\n" not in response:
        chunk = sock.recv(4096)
        if not chunk:
            sock.close()
            raise LauncherError("browser debugging handshake failed")
        response.extend(chunk)

    header_text = response.split(b"\r\n\r\n", 1)[0].decode("utf-8", errors="replace")
    if "101" not in header_text.splitlines()[0]:
        sock.close()
        raise LauncherError("browser debugging handshake failed")

    expected_accept = base64.b64encode(
        hashlib.sha1((websocket_key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode("ascii")).digest()
    ).decode("ascii")
    if f"Sec-WebSocket-Accept: {expected_accept}" not in header_text:
        sock.close()
        raise LauncherError("browser debugging handshake was rejected")

    return sock


def _cdp_send(sock: socket.socket, message_id: int, method: str, params: Optional[Mapping[str, Any]] = None) -> None:
    payload = {"id": message_id, "method": method}
    if params:
        payload["params"] = dict(params)
    _send_websocket_frame(sock, json.dumps(payload))


def _cdp_wait_for_message(
    sock: socket.socket,
    *,
    timeout_seconds: float,
    expected_id: Optional[int] = None,
    expected_method: Optional[str] = None,
) -> Mapping[str, Any]:
    deadline = time.monotonic() + timeout_seconds
    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise LauncherError("timed out while waiting for browser diagnostics")
        sock.settimeout(remaining)
        message = json.loads(_recv_websocket_frame(sock))
        if expected_id is None and expected_method is None:
            return message
        if expected_id is not None and message.get("id") == expected_id:
            return message
        if expected_method is not None and message.get("method") == expected_method:
            return message


def _cdp_request(
    sock: socket.socket,
    message_id: int,
    method: str,
    params: Optional[Mapping[str, Any]],
    timeout_seconds: float,
) -> Mapping[str, Any]:
    _cdp_send(sock, message_id, method, params)
    response = _cdp_wait_for_message(sock, timeout_seconds=timeout_seconds, expected_id=message_id)
    error = response.get("error")
    if isinstance(error, Mapping):
        detail = error.get("message") or "unknown browser debugging error"
        raise LauncherError(str(detail))
    return response


def _evaluate_browser_expression(sock: socket.socket, message_id: int, expression: str, timeout_seconds: float) -> Tuple[int, str]:
    response = _cdp_request(
        sock,
        message_id,
        "Runtime.evaluate",
        {
            "expression": expression,
            "returnByValue": True,
            "awaitPromise": True,
        },
        timeout_seconds,
    )
    result = response.get("result")
    if not isinstance(result, Mapping):
        raise LauncherError("browser returned an unreadable evaluation result")

    exception_details = result.get("exceptionDetails")
    if isinstance(exception_details, Mapping):
        detail = exception_details.get("text")
        raise LauncherError(str(detail or "browser evaluation failed"))

    value_container = result.get("result")
    if not isinstance(value_container, Mapping):
        return message_id + 1, ""

    value = value_container.get("value")
    if value is None:
        return message_id + 1, ""
    return message_id + 1, str(value)


def _run_browser_smoke_check_via_cdp(
    url: str,
    *,
    chrome_process: subprocess.Popen,
    profile_dir: Path,
    timeout_seconds: float,
) -> BrowserSmokeCheckResult:
    try:
        browser_target = _wait_for_devtools_target(profile_dir, chrome_process=chrome_process, timeout_seconds=timeout_seconds)
        page_target = _wait_for_page_target(
            browser_target.port,
            chrome_process=chrome_process,
            timeout_seconds=timeout_seconds,
        )
    except LauncherError as exc:
        return BrowserSmokeCheckResult(status="browser_error", detail=str(exc))

    sock: Optional[socket.socket] = None
    message_id = 1
    first_exception_detail = ""
    try:
        sock = _connect_to_devtools(page_target.websocket_url, timeout_seconds=timeout_seconds)
        _cdp_request(sock, message_id, "Runtime.enable", None, timeout_seconds)
        message_id += 1
        _cdp_request(sock, message_id, "Page.enable", None, timeout_seconds)
        message_id += 1
        _cdp_request(sock, message_id, "Page.navigate", {"url": url}, timeout_seconds)
        message_id += 1

        deadline = time.monotonic() + timeout_seconds
        load_event_seen = False
        while not load_event_seen:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                return BrowserSmokeCheckResult(status="failed", detail="page load timed out")
            message = _cdp_wait_for_message(sock, timeout_seconds=remaining)
            method = message.get("method")
            if method == "Runtime.exceptionThrown" and not first_exception_detail:
                first_exception_detail = _extract_exception_detail(message) or "Runtime.exceptionThrown"
            elif method == "Page.loadEventFired":
                load_event_seen = True

        settle_deadline = min(deadline, time.monotonic() + min(DEFAULT_BROWSER_SETTLE_WINDOW_SECONDS, timeout_seconds))
        while True:
            remaining = settle_deadline - time.monotonic()
            if remaining <= 0:
                break
            try:
                message = _cdp_wait_for_message(sock, timeout_seconds=remaining)
            except (LauncherError, TimeoutError, socket.timeout):
                break

            method = message.get("method")
            if method == "Runtime.exceptionThrown" and not first_exception_detail:
                first_exception_detail = _extract_exception_detail(message) or "Runtime.exceptionThrown"

        message_id, page_title = _evaluate_browser_expression(sock, message_id, "document.title", timeout_seconds)
        message_id, root_html = _evaluate_browser_expression(
            sock,
            message_id,
            "(function(){const root=document.querySelector('#root'); return root ? root.innerHTML : '';})()",
            timeout_seconds,
        )
        _, body_text = _evaluate_browser_expression(
            sock,
            message_id,
            "(function(){return document.body ? document.body.innerText : '';})()",
            timeout_seconds,
        )

        if first_exception_detail:
            return BrowserSmokeCheckResult(status="failed", detail=first_exception_detail)
        _ = page_title
        if not root_html.strip():
            return BrowserSmokeCheckResult(status="failed", detail="root element remained empty after page load")
        if not body_text.strip():
            return BrowserSmokeCheckResult(status="failed", detail="document body text remained empty after page load")
        return BrowserSmokeCheckResult(status="passed")
    except (LauncherError, OSError, json.JSONDecodeError) as exc:
        return BrowserSmokeCheckResult(status="failed", detail=str(exc))
    finally:
        if sock is not None:
            try:
                sock.close()
            except OSError:
                pass


def run_browser_smoke_check(url: str, *, output: Optional[TextIO] = None, timeout_seconds: float = 15.0) -> None:
    stream = output or sys.stdout
    browser = find_browser_executable()
    if not browser:
        stream.write(f"[warn] Browser smoke check skipped for {url}: Chrome/Chromium not found\n")
        return

    profile_dir = Path(tempfile.mkdtemp(prefix="launcher-browser-smoke-"))
    chrome_process: Optional[subprocess.Popen] = None
    try:
        chrome_process = subprocess.Popen(
            [
                browser,
                f"--user-data-dir={profile_dir}",
                "--headless=new",
                "--disable-gpu",
                "--remote-debugging-port=0",
                "about:blank",
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            text=True,
        )
        result = _run_browser_smoke_check_via_cdp(
            url,
            chrome_process=chrome_process,
            profile_dir=profile_dir,
            timeout_seconds=timeout_seconds,
        )
        if result.status == "browser_error":
            raise LauncherError(
                _build_actionable_message(
                    f"Browser smoke check could not start a usable Chrome/Chromium session for {url}: {result.detail}",
                    "Verify Chrome/Chromium is installed and launchable, then retry the launcher.",
                )
            )
        if result.status != "passed":
            raise LauncherError(
                _build_actionable_message(
                    f"Browser smoke check failed for {url}: {result.detail}",
                    "Open the URL in a clean browser profile or inspect the built frontend bundle for startup errors.",
                )
            )
        stream.write(f"[ok] Browser smoke check passed: {url}\n")
    except OSError as exc:
        raise LauncherError(
            _build_actionable_message(
                f"Browser smoke check failed for {url}: browser could not start.",
                "Verify Chrome/Chromium is installed and launchable.",
            )
        ) from exc
    finally:
        if chrome_process is not None and chrome_process.poll() is None:
            chrome_process.terminate()
            try:
                chrome_process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                chrome_process.kill()
        shutil.rmtree(profile_dir, ignore_errors=True)


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
