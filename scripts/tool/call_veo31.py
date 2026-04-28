#!/usr/bin/env python3
"""Submit a VEO 3.1 video generation request through HermesToken.

The submit URL is intentionally fixed to the production HermesToken endpoint.
The API key is requested interactively by default so it is not stored in this
file or echoed back to the terminal.
"""

from __future__ import annotations

import argparse
import getpass
import json
import mimetypes
import os
import sys
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence, TextIO, Tuple
from urllib import error, request
from urllib.parse import urljoin, urlparse

try:
    import requests
except ImportError:  # pragma: no cover - urllib fallback is kept for bare Python installs.
    requests = None


DEFAULT_BASE_URL = "https://hermestoken.top"
DEFAULT_ENDPOINT = "/v1/video/generations"
DEFAULT_MODEL = "veo_3_1"
MODEL_CHOICES = ("veo_3_1", "veo_3_1-4K")
DEFAULT_SIZE = "720x1280"
SIZE_CHOICES = ("720x1280", "1280x720")
DEFAULT_SECONDS = 8
DEFAULT_OUTPUT = str(Path(__file__).resolve().with_name("veo31_output.mp4"))
DEFAULT_POLL_TIMEOUT = 900.0
DEFAULT_POLL_INTERVAL = 5.0
DEFAULT_REQUEST_TIMEOUT = 60.0
SUCCESS_STATUSES = {"succeeded", "success", "completed", "complete", "done", "finished"}
FAILURE_STATUSES = {"failed", "failure", "error", "errored", "cancelled", "canceled"}


@dataclass(frozen=True)
class FilePart:
    field_name: str
    path: Path
    content_type: str


def normalize_base_url(base_url: str) -> str:
    normalized = base_url.strip().rstrip("/")
    if not normalized:
        raise ValueError("base URL cannot be empty")
    parsed = urlparse(normalized)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise ValueError(f"invalid base URL: {base_url}")
    return normalized


def normalize_endpoint(endpoint: str) -> str:
    stripped = endpoint.strip()
    if not stripped:
        raise ValueError("endpoint cannot be empty")
    return stripped if stripped.startswith("/") else f"/{stripped}"


def build_submit_url(base_url: str, endpoint: str = DEFAULT_ENDPOINT) -> str:
    return f"{normalize_base_url(base_url)}{normalize_endpoint(endpoint)}"


def build_poll_url(base_url: str, endpoint: str, task_id: str) -> str:
    endpoint = normalize_endpoint(endpoint)
    if endpoint == "/v1/videos":
        poll_endpoint = f"/v1/videos/{task_id}"
    elif endpoint == "/v1/video/generations":
        poll_endpoint = f"/v1/video/generations/{task_id}"
    else:
        poll_endpoint = f"{endpoint.rstrip('/')}/{task_id}"
    return f"{normalize_base_url(base_url)}{poll_endpoint}"


def redact_secret(secret: str) -> str:
    if len(secret) <= 8:
        return "***"
    return f"{secret[:3]}...{secret[-4:]}"


def build_multipart_form_data(
    *,
    fields: Dict[str, str],
    files: Iterable[FilePart],
    boundary: Optional[str] = None,
) -> Tuple[bytes, str]:
    boundary = boundary or f"----hermestoken-veo31-{uuid.uuid4().hex}"
    chunks: List[bytes] = []

    for name, value in fields.items():
        chunks.extend(
            [
                f"--{boundary}\r\n".encode("utf-8"),
                f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode("utf-8"),
                str(value).encode("utf-8"),
                b"\r\n",
            ]
        )

    for file_part in files:
        filename = file_part.path.name
        chunks.extend(
            [
                f"--{boundary}\r\n".encode("utf-8"),
                (
                    f'Content-Disposition: form-data; name="{file_part.field_name}"; '
                    f'filename="{filename}"\r\n'
                ).encode("utf-8"),
                f"Content-Type: {file_part.content_type}\r\n\r\n".encode("utf-8"),
                file_part.path.read_bytes(),
                b"\r\n",
            ]
        )

    chunks.append(f"--{boundary}--\r\n".encode("utf-8"))
    return b"".join(chunks), f"multipart/form-data; boundary={boundary}"


def http_request_json(
    url: str,
    method: str,
    headers: Dict[str, str],
    body: Optional[bytes] = None,
    *,
    timeout: float = 60.0,
) -> Any:
    if requests is not None:
        try:
            response = requests.request(method, url, headers=headers, data=body, timeout=timeout)
        except requests.RequestException as exc:
            raise RuntimeError(f"request failed for {url}: {exc}") from exc

        if response.status_code >= 400:
            raise RuntimeError(f"HTTP {response.status_code} from {url}: {response.text}")

        if not response.content:
            return {}

        try:
            return response.json()
        except ValueError:
            return {"raw": response.text}

    req = request.Request(url, data=body, headers=headers, method=method)
    try:
        with request.urlopen(req, timeout=timeout) as response:
            raw = response.read()
    except error.HTTPError as exc:
        raw = exc.read()
        message = raw.decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {exc.code} from {url}: {message}") from exc
    except error.URLError as exc:
        raise RuntimeError(f"request failed for {url}: {exc.reason}") from exc

    if not raw:
        return {}

    text = raw.decode("utf-8", errors="replace")
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return {"raw": text}


def _first_string_for_keys(value: Any, keys: Sequence[str]) -> Optional[str]:
    if isinstance(value, dict):
        for key in keys:
            candidate = value.get(key)
            if isinstance(candidate, str) and candidate.strip():
                return candidate.strip()
            if isinstance(candidate, (int, float)):
                return str(candidate)
        for child in value.values():
            nested = _first_string_for_keys(child, keys)
            if nested:
                return nested
    elif isinstance(value, list):
        for item in value:
            nested = _first_string_for_keys(item, keys)
            if nested:
                return nested
    return None


def extract_task_id(response_payload: Any) -> Optional[str]:
    return _first_string_for_keys(response_payload, ("task_id", "video_id", "id"))


def extract_status(response_payload: Any) -> Optional[str]:
    status = _first_string_for_keys(response_payload, ("status", "state"))
    return status.lower() if status else None


def extract_video_url(response_payload: Any) -> Optional[str]:
    return _first_string_for_keys(
        response_payload,
        (
            "video_url",
            "output_url",
            "download_url",
            "url",
            "content_url",
        ),
    )


def load_input_reference(path: Optional[str]) -> List[FilePart]:
    if not path:
        return []
    file_path = Path(path).expanduser()
    if not file_path.is_file():
        raise FileNotFoundError(f"input reference file not found: {file_path}")
    content_type = mimetypes.guess_type(str(file_path))[0] or "application/octet-stream"
    return [FilePart(field_name="input_reference", path=file_path, content_type=content_type)]


def _read_line(prompt: str, stdin: TextIO, output: TextIO) -> str:
    print(prompt, end="", file=output, flush=True)
    raw = stdin.readline()
    if raw == "":
        raise EOFError("interactive input ended unexpectedly")
    return raw.strip()


def prompt_text(
    label: str,
    *,
    default: Optional[str] = None,
    required: bool = False,
    secret: bool = False,
    stdin: TextIO = sys.stdin,
    output: TextIO = sys.stderr,
) -> str:
    suffix = f" [{default}]" if default not in (None, "") else ""
    prompt = f"{label}{suffix}: "
    while True:
        raw = (
            getpass.getpass(prompt, stream=output).strip()
            if secret
            else _read_line(prompt, stdin, output)
        )
        if raw:
            return raw
        if default is not None:
            return default
        if not required:
            return ""
        print(f"{label}不能为空，请重新输入。", file=output)


def prompt_option(
    label: str,
    choices: Sequence[str],
    *,
    default: str,
    stdin: TextIO,
    output: TextIO,
) -> str:
    print(f"{label}:", file=output)
    for index, choice in enumerate(choices, start=1):
        marker = "（默认）" if choice == default else ""
        print(f"  {index}. {choice}{marker}", file=output)
    default_index = choices.index(default) + 1
    while True:
        value = prompt_text("请选择", default=str(default_index), stdin=stdin, output=output)
        if value.isdigit():
            option_index = int(value)
            if 1 <= option_index <= len(choices):
                return choices[option_index - 1]
        if value in choices:
            return value
        print(f"请输入 1-{len(choices)} 的编号，或直接输入选项值。", file=output)


def prompt_bool(label: str, *, default: bool, stdin: TextIO, output: TextIO) -> bool:
    suffix = "Y/n" if default else "y/N"
    while True:
        value = prompt_text(f"{label} ({suffix})", default="", stdin=stdin, output=output).lower()
        if not value:
            return default
        if value in {"y", "yes", "true", "1", "是"}:
            return True
        if value in {"n", "no", "false", "0", "否"}:
            return False
        print("请输入 y 或 n。", file=output)


def prompt_positive_int(label: str, *, default: int, stdin: TextIO, output: TextIO) -> int:
    while True:
        value = prompt_text(label, default=str(default), stdin=stdin, output=output)
        try:
            parsed = int(value)
        except ValueError:
            print("请输入整数。", file=output)
            continue
        if parsed > 0:
            return parsed
        print("请输入大于 0 的整数。", file=output)


def prompt_positive_float(label: str, *, default: float, stdin: TextIO, output: TextIO) -> float:
    while True:
        value = prompt_text(label, default=str(default), stdin=stdin, output=output)
        try:
            parsed = float(value)
        except ValueError:
            print("请输入数字。", file=output)
            continue
        if parsed > 0:
            return parsed
        print("请输入大于 0 的数字。", file=output)


def is_interactive_stdin(stdin: TextIO) -> bool:
    isatty = getattr(stdin, "isatty", None)
    return bool(isatty and isatty())


def apply_default_args(args: argparse.Namespace) -> argparse.Namespace:
    values = {
        **vars(args),
        "api_key": args.api_key or os.getenv("HERMESTOKEN_API_KEY"),
        "model": args.model or DEFAULT_MODEL,
        "size": args.size or DEFAULT_SIZE,
        "seconds": args.seconds if args.seconds is not None else DEFAULT_SECONDS,
        "enable_upsample": bool(args.enable_upsample) if args.enable_upsample is not None else False,
        "no_poll": bool(args.no_poll) if args.no_poll is not None else False,
        "no_download": bool(args.no_download) if args.no_download is not None else False,
        "output": args.output or DEFAULT_OUTPUT,
        "poll_timeout": args.poll_timeout if args.poll_timeout is not None else DEFAULT_POLL_TIMEOUT,
        "poll_interval": args.poll_interval if args.poll_interval is not None else DEFAULT_POLL_INTERVAL,
        "request_timeout": args.request_timeout if args.request_timeout is not None else DEFAULT_REQUEST_TIMEOUT,
        "dry_run": bool(args.dry_run) if args.dry_run is not None else False,
    }
    return argparse.Namespace(**values)


def prompt_for_args(args: argparse.Namespace, *, stdin: TextIO, output: TextIO) -> argparse.Namespace:
    values = vars(args).copy()
    if not values.get("api_key"):
        values["api_key"] = prompt_text("API key", required=True, secret=True, output=output)
    if not values.get("model"):
        values["model"] = prompt_option(
            "模型",
            MODEL_CHOICES,
            default=DEFAULT_MODEL,
            stdin=stdin,
            output=output,
        )
    if not values.get("prompt"):
        values["prompt"] = prompt_text("提示词", required=True, stdin=stdin, output=output)
    if not values.get("size"):
        values["size"] = prompt_option(
            "尺寸",
            SIZE_CHOICES,
            default=DEFAULT_SIZE,
            stdin=stdin,
            output=output,
        )
    if values.get("seconds") is None:
        values["seconds"] = prompt_positive_int("秒数", default=DEFAULT_SECONDS, stdin=stdin, output=output)
    if values.get("input_reference") is None:
        input_reference = prompt_text("参考图路径（可空）", default="", stdin=stdin, output=output)
        values["input_reference"] = input_reference or None
    if values.get("enable_upsample") is None:
        values["enable_upsample"] = prompt_bool("是否启用 upsample", default=False, stdin=stdin, output=output)
    if values.get("dry_run") is None:
        values["dry_run"] = prompt_bool("只打印请求不发送", default=False, stdin=stdin, output=output)
    if not values.get("dry_run"):
        if values.get("no_poll") is None:
            values["no_poll"] = not prompt_bool("提交后轮询直到完成", default=True, stdin=stdin, output=output)
        if values.get("no_download") is None:
            should_download = (
                False
                if values.get("no_poll")
                else prompt_bool("完成后下载视频", default=True, stdin=stdin, output=output)
            )
            values["no_download"] = not should_download
        if not values.get("no_download") and not values.get("output"):
            values["output"] = prompt_text("下载输出路径", default=DEFAULT_OUTPUT, stdin=stdin, output=output)
        if not values.get("no_poll") and values.get("poll_timeout") is None:
            values["poll_timeout"] = prompt_positive_float(
                "轮询超时秒数",
                default=DEFAULT_POLL_TIMEOUT,
                stdin=stdin,
                output=output,
            )
        if not values.get("no_poll") and values.get("poll_interval") is None:
            values["poll_interval"] = prompt_positive_float(
                "轮询间隔秒数",
                default=DEFAULT_POLL_INTERVAL,
                stdin=stdin,
                output=output,
            )
        if values.get("request_timeout") is None:
            values["request_timeout"] = prompt_positive_float(
                "单次请求超时秒数",
                default=DEFAULT_REQUEST_TIMEOUT,
                stdin=stdin,
                output=output,
            )
    return apply_default_args(argparse.Namespace(**values))


def validate_args(args: argparse.Namespace) -> Optional[str]:
    if not args.api_key:
        return "missing API key. Pass --api-key or run interactively."
    if not args.prompt:
        return "missing prompt. Pass --prompt or run interactively."
    if args.model not in MODEL_CHOICES:
        return f"invalid model. Use one of: {', '.join(MODEL_CHOICES)}."
    if args.size not in SIZE_CHOICES:
        return "invalid size. Use 720x1280 or 1280x720."
    if args.seconds <= 0:
        return "seconds must be greater than 0."
    if args.poll_timeout <= 0 or args.poll_interval <= 0 or args.request_timeout <= 0:
        return "timeouts and poll interval must be greater than 0."
    return None


def make_request_fields(args: argparse.Namespace) -> Dict[str, str]:
    fields = {
        "model": args.model,
        "prompt": args.prompt,
        "size": args.size,
        "seconds": str(args.seconds),
    }
    if args.enable_upsample:
        fields["enable_upsample"] = "true"
    return fields


def poll_until_done(
    *,
    base_url: str,
    endpoint: str,
    task_id: str,
    api_key: str,
    timeout_seconds: float,
    interval_seconds: float,
    request_timeout: float,
    output: TextIO,
) -> Any:
    deadline = time.monotonic() + timeout_seconds
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Accept": "application/json",
        "User-Agent": "hermestoken-veo31-python/1.0",
    }
    poll_url = build_poll_url(base_url, endpoint, task_id)

    while True:
        payload = http_request_json(poll_url, "GET", headers, None, timeout=request_timeout)
        status = extract_status(payload)
        video_url = extract_video_url(payload)
        if status in SUCCESS_STATUSES or (status is None and video_url):
            print(f"[ok] task finished: {task_id}", file=output)
            return payload
        if status in FAILURE_STATUSES:
            raise RuntimeError(f"video task failed: {json.dumps(payload, ensure_ascii=False)}")
        if time.monotonic() >= deadline:
            raise TimeoutError(f"timed out waiting for video task {task_id}")
        print(f"[wait] task={task_id} status={status or 'unknown'}", file=output)
        time.sleep(interval_seconds)


def should_send_auth_to_download(download_url: str, base_url: str) -> bool:
    parsed_download = urlparse(download_url)
    parsed_base = urlparse(normalize_base_url(base_url))
    if not parsed_download.netloc:
        return True
    return parsed_download.netloc == parsed_base.netloc


def download_video(
    *,
    video_url: str,
    base_url: str,
    api_key: str,
    output_path: str,
    timeout: float,
) -> Path:
    resolved_url = urljoin(f"{normalize_base_url(base_url)}/", video_url)
    headers = {"User-Agent": "hermestoken-veo31-python/1.0"}
    if should_send_auth_to_download(resolved_url, base_url):
        headers["Authorization"] = f"Bearer {api_key}"
    destination = Path(output_path).expanduser()
    destination.parent.mkdir(parents=True, exist_ok=True)

    if requests is not None:
        try:
            response = requests.get(resolved_url, headers=headers, timeout=timeout)
        except requests.RequestException as exc:
            raise RuntimeError(f"video download failed: {exc}") from exc
        if response.status_code >= 400:
            raise RuntimeError(f"video download failed with HTTP {response.status_code}")
        destination.write_bytes(response.content)
        return destination

    req = request.Request(resolved_url, headers=headers, method="GET")
    try:
        with request.urlopen(req, timeout=timeout) as response:
            destination.write_bytes(response.read())
    except error.HTTPError as exc:
        raise RuntimeError(f"video download failed with HTTP {exc.code}") from exc
    except error.URLError as exc:
        raise RuntimeError(f"video download failed: {exc.reason}") from exc
    return destination


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Call VEO 3.1 through HermesToken.")
    parser.add_argument("--api-key", help="API key. Omit it to enter the key interactively.")
    parser.add_argument("--base-url", help=argparse.SUPPRESS)
    parser.add_argument("--endpoint", help=argparse.SUPPRESS)
    parser.add_argument("--interactive", action="store_true", help="Prompt for missing values.")
    parser.add_argument("--non-interactive", action="store_true", help="Never prompt for missing values.")
    parser.add_argument("--model", choices=MODEL_CHOICES)
    parser.add_argument("--prompt")
    parser.add_argument("--size", choices=SIZE_CHOICES)
    parser.add_argument("--seconds", type=int)
    parser.add_argument("--input-reference", help="Optional image reference file path.")
    parser.add_argument(
        "--enable-upsample",
        dest="enable_upsample",
        action="store_true",
        help="Enable upsample for supported horizontal videos.",
    )
    parser.add_argument(
        "--no-enable-upsample",
        dest="enable_upsample",
        action="store_false",
        help="Disable upsample.",
    )
    parser.add_argument(
        "--poll",
        dest="no_poll",
        action="store_false",
        help="Poll until the task finishes.",
    )
    parser.add_argument(
        "--no-poll",
        dest="no_poll",
        action="store_true",
        help="Only submit the task and print the task id.",
    )
    parser.add_argument(
        "--download",
        dest="no_download",
        action="store_false",
        help="Download the finished video.",
    )
    parser.add_argument(
        "--no-download",
        dest="no_download",
        action="store_true",
        help="Do not download the finished video.",
    )
    parser.add_argument("--output", help="Downloaded video path.")
    parser.add_argument("--poll-timeout", type=float)
    parser.add_argument("--poll-interval", type=float)
    parser.add_argument("--request-timeout", type=float)
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the request shape without sending it.",
    )
    parser.set_defaults(enable_upsample=None, no_poll=None, no_download=None, dry_run=None)
    return parser


def main(
    argv: Optional[Sequence[str]] = None,
    *,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
    stdin: TextIO = sys.stdin,
) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        should_prompt = args.interactive or (
            argv is None and not args.non_interactive and is_interactive_stdin(stdin)
        )
        args = prompt_for_args(args, stdin=stdin, output=stderr) if should_prompt else apply_default_args(args)
        validation_error = validate_args(args)
        if validation_error:
            print(f"error: {validation_error}", file=stderr)
            return 2

        base_url = DEFAULT_BASE_URL
        endpoint = DEFAULT_ENDPOINT
        api_key = args.api_key
        fields = make_request_fields(args)
        files = load_input_reference(args.input_reference)
        body, content_type = build_multipart_form_data(fields=fields, files=files)
        headers = {
            "Authorization": f"Bearer {api_key}",
            "Accept": "application/json",
            "Content-Type": content_type,
            "User-Agent": "hermestoken-veo31-python/1.0",
        }
        submit_url = build_submit_url(base_url, endpoint)

        if args.dry_run:
            print(f"POST {submit_url}", file=stdout)
            print(f"Authorization: Bearer {redact_secret(api_key)}", file=stdout)
            print(f"Content-Type: {content_type}", file=stdout)
            print("fields:", file=stdout)
            print(json.dumps(fields, ensure_ascii=False, indent=2), file=stdout)
            if files:
                print("files:", file=stdout)
                for file_part in files:
                    print(f"- {file_part.field_name}: {file_part.path} ({file_part.content_type})", file=stdout)
            print(f"multipart_bytes={len(body)}", file=stdout)
            return 0

        submit_payload = http_request_json(submit_url, "POST", headers, body, timeout=args.request_timeout)
        print(json.dumps(submit_payload, ensure_ascii=False, indent=2), file=stdout)

        task_id = extract_task_id(submit_payload)
        video_url = extract_video_url(submit_payload)
        if args.no_poll:
            if task_id:
                print(f"[ok] submitted task_id={task_id}", file=stdout)
            return 0

        final_payload = submit_payload
        if task_id and not video_url:
            final_payload = poll_until_done(
                base_url=base_url,
                endpoint=endpoint,
                task_id=task_id,
                api_key=api_key,
                timeout_seconds=args.poll_timeout,
                interval_seconds=args.poll_interval,
                request_timeout=args.request_timeout,
                output=stdout,
            )
            print(json.dumps(final_payload, ensure_ascii=False, indent=2), file=stdout)

        video_url = extract_video_url(final_payload)
        if not video_url:
            print("error: request completed but no video URL was found in the response.", file=stderr)
            return 3

        print(f"[ok] video_url={video_url}", file=stdout)
        if not args.no_download:
            destination = download_video(
                video_url=video_url,
                base_url=base_url,
                api_key=api_key,
                output_path=args.output,
                timeout=args.request_timeout,
            )
            print(f"[ok] downloaded={destination}", file=stdout)
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
