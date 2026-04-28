#!/usr/bin/env python3
"""Generate an image with openai/gpt-image-2 through HermesToken.

The submit URL is fixed to the production HermesToken image endpoint.
The API key is requested interactively by default so it is not stored in this
file or echoed back to the terminal.
"""

from __future__ import annotations

import argparse
import base64
import getpass
import json
import os
import sys
import time
from pathlib import Path
from typing import Any, Dict, List, Optional, Sequence, TextIO, Tuple
from urllib import error, request
from urllib.parse import urljoin, urlparse

try:
    import requests
except ImportError:  # pragma: no cover - urllib fallback is kept for bare Python installs.
    requests = None


DEFAULT_BASE_URL = "https://hermestoken.top"
DEFAULT_ENDPOINT = "/v1/images/generations"
DEFAULT_MODEL = "openai/gpt-image-2"
MODEL_CHOICES = (DEFAULT_MODEL,)
DEFAULT_SIZE = "1024x1024"
SIZE_CHOICES = ("1024x1024", "1024x1536", "1536x1024", "auto")
DEFAULT_QUALITY = "auto"
QUALITY_CHOICES = ("auto", "low", "medium", "high")
DEFAULT_RESPONSE_FORMAT = "b64_json"
RESPONSE_FORMAT_CHOICES = ("b64_json", "url")
DEFAULT_OUTPUT_FORMAT = "png"
OUTPUT_FORMAT_CHOICES = ("png", "jpeg", "webp")
DEFAULT_OUTPUT = str(Path(__file__).resolve().with_name("gpt_image2_output.png"))
DEFAULT_REQUEST_TIMEOUT = 120.0


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


def redact_secret(secret: str) -> str:
    if len(secret) <= 8:
        return "***"
    return f"{secret[:3]}...{secret[-4:]}"


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
        "quality": args.quality or DEFAULT_QUALITY,
        "n": args.n if args.n is not None else 1,
        "response_format": args.response_format or DEFAULT_RESPONSE_FORMAT,
        "output_format": args.output_format or DEFAULT_OUTPUT_FORMAT,
        "output": args.output or default_output_for_format(DEFAULT_OUTPUT_FORMAT),
        "request_timeout": (
            args.request_timeout if args.request_timeout is not None else DEFAULT_REQUEST_TIMEOUT
        ),
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
    if not values.get("quality"):
        values["quality"] = prompt_option(
            "质量",
            QUALITY_CHOICES,
            default=DEFAULT_QUALITY,
            stdin=stdin,
            output=output,
        )
    if values.get("n") is None:
        values["n"] = prompt_positive_int("生成张数", default=1, stdin=stdin, output=output)
    if not values.get("response_format"):
        values["response_format"] = prompt_option(
            "返回格式",
            RESPONSE_FORMAT_CHOICES,
            default=DEFAULT_RESPONSE_FORMAT,
            stdin=stdin,
            output=output,
        )
    if not values.get("output_format"):
        values["output_format"] = prompt_option(
            "图片格式",
            OUTPUT_FORMAT_CHOICES,
            default=DEFAULT_OUTPUT_FORMAT,
            stdin=stdin,
            output=output,
        )
    if not values.get("output"):
        default_output = default_output_for_format(values["output_format"])
        values["output"] = prompt_text("保存路径", default=default_output, stdin=stdin, output=output)
    if values.get("request_timeout") is None:
        values["request_timeout"] = prompt_positive_float(
            "请求超时秒数",
            default=DEFAULT_REQUEST_TIMEOUT,
            stdin=stdin,
            output=output,
        )
    if values.get("dry_run") is None:
        values["dry_run"] = prompt_bool("只打印请求不发送", default=False, stdin=stdin, output=output)
    return apply_default_args(argparse.Namespace(**values))


def default_output_for_format(output_format: str) -> str:
    suffix = "jpg" if output_format == "jpeg" else output_format
    return str(Path(__file__).resolve().with_name(f"gpt_image2_output.{suffix}"))


def validate_args(args: argparse.Namespace) -> Optional[str]:
    if not args.api_key:
        return "missing API key. Pass --api-key or run interactively."
    if not args.prompt:
        return "missing prompt. Pass --prompt or run interactively."
    if args.model not in MODEL_CHOICES:
        return f"invalid model. Use one of: {', '.join(MODEL_CHOICES)}."
    if args.size not in SIZE_CHOICES:
        return f"invalid size. Use one of: {', '.join(SIZE_CHOICES)}."
    if args.quality not in QUALITY_CHOICES:
        return f"invalid quality. Use one of: {', '.join(QUALITY_CHOICES)}."
    if args.response_format not in RESPONSE_FORMAT_CHOICES:
        return f"invalid response format. Use one of: {', '.join(RESPONSE_FORMAT_CHOICES)}."
    if args.output_format not in OUTPUT_FORMAT_CHOICES:
        return f"invalid output format. Use one of: {', '.join(OUTPUT_FORMAT_CHOICES)}."
    if args.n <= 0:
        return "n must be greater than 0."
    if args.request_timeout <= 0:
        return "request timeout must be greater than 0."
    return None


def make_request_payload(args: argparse.Namespace) -> Dict[str, Any]:
    return {
        "model": args.model,
        "prompt": args.prompt,
        "size": args.size,
        "quality": args.quality,
        "n": args.n,
        "response_format": args.response_format,
        "output_format": args.output_format,
    }


def http_request_json(
    url: str,
    method: str,
    headers: Dict[str, str],
    body: Optional[bytes] = None,
    *,
    timeout: float = DEFAULT_REQUEST_TIMEOUT,
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


def _collect_image_items(value: Any, items: List[Dict[str, str]]) -> None:
    if isinstance(value, dict):
        item: Dict[str, str] = {}
        url = value.get("url") or value.get("image_url")
        b64_json = value.get("b64_json") or value.get("base64") or value.get("image")
        if isinstance(url, str) and url.strip():
            item["url"] = url.strip()
        if isinstance(b64_json, str) and b64_json.strip():
            item["b64_json"] = strip_data_url_prefix(b64_json.strip())
        if item:
            items.append(item)
            return
        for child in value.values():
            _collect_image_items(child, items)
    elif isinstance(value, list):
        for child in value:
            _collect_image_items(child, items)


def extract_image_items(response_payload: Any) -> List[Dict[str, str]]:
    items: List[Dict[str, str]] = []
    _collect_image_items(response_payload, items)
    return items


def strip_data_url_prefix(value: str) -> str:
    if value.startswith("data:") and "," in value:
        return value.split(",", 1)[1]
    return value


def output_path_for_index(output_path: str, index: int, total: int) -> Path:
    destination = Path(output_path).expanduser()
    if total <= 1:
        return destination
    return destination.with_name(f"{destination.stem}_{index}{destination.suffix}")


def should_send_auth_to_download(download_url: str, base_url: str) -> bool:
    parsed_download = urlparse(download_url)
    parsed_base = urlparse(normalize_base_url(base_url))
    if not parsed_download.netloc:
        return True
    return parsed_download.netloc == parsed_base.netloc


def download_file(
    *,
    file_url: str,
    base_url: str,
    api_key: str,
    output_path: Path,
    timeout: float,
) -> Path:
    resolved_url = urljoin(f"{normalize_base_url(base_url)}/", file_url)
    headers = {"User-Agent": "hermestoken-gpt-image2-python/1.0"}
    if should_send_auth_to_download(resolved_url, base_url):
        headers["Authorization"] = f"Bearer {api_key}"
    output_path.parent.mkdir(parents=True, exist_ok=True)

    if requests is not None:
        try:
            response = requests.get(resolved_url, headers=headers, timeout=timeout)
        except requests.RequestException as exc:
            raise RuntimeError(f"image download failed: {exc}") from exc
        if response.status_code >= 400:
            raise RuntimeError(f"image download failed with HTTP {response.status_code}")
        output_path.write_bytes(response.content)
        return output_path

    req = request.Request(resolved_url, headers=headers, method="GET")
    try:
        with request.urlopen(req, timeout=timeout) as response:
            output_path.write_bytes(response.read())
    except error.HTTPError as exc:
        raise RuntimeError(f"image download failed with HTTP {exc.code}") from exc
    except error.URLError as exc:
        raise RuntimeError(f"image download failed: {exc.reason}") from exc
    return output_path


def save_base64_image(encoded_image: str, output_path: Path) -> Path:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    try:
        output_path.write_bytes(base64.b64decode(strip_data_url_prefix(encoded_image), validate=True))
    except ValueError as exc:
        raise RuntimeError("image response contains invalid base64 data") from exc
    return output_path


def save_images(
    *,
    response_payload: Any,
    base_url: str,
    api_key: str,
    output_path: str,
    timeout: float,
) -> List[Path]:
    items = extract_image_items(response_payload)
    if not items:
        raise RuntimeError("request completed but no image URL or base64 image was found in the response")

    saved_paths: List[Path] = []
    total = len(items)
    for index, item in enumerate(items, start=1):
        destination = output_path_for_index(output_path, index, total)
        if "b64_json" in item:
            saved_paths.append(save_base64_image(item["b64_json"], destination))
        elif "url" in item:
            saved_paths.append(
                download_file(
                    file_url=item["url"],
                    base_url=base_url,
                    api_key=api_key,
                    output_path=destination,
                    timeout=timeout,
                )
            )
    return saved_paths


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Generate an image with openai/gpt-image-2 through HermesToken.")
    parser.add_argument("--api-key", help="API key. Omit it to enter the key interactively.")
    parser.add_argument("--base-url", help=argparse.SUPPRESS)
    parser.add_argument("--endpoint", help=argparse.SUPPRESS)
    parser.add_argument("--interactive", action="store_true", help="Prompt for missing values.")
    parser.add_argument("--non-interactive", action="store_true", help="Never prompt for missing values.")
    parser.add_argument("--model", choices=MODEL_CHOICES)
    parser.add_argument("--prompt")
    parser.add_argument("--size", choices=SIZE_CHOICES)
    parser.add_argument("--quality", choices=QUALITY_CHOICES)
    parser.add_argument("--n", type=int)
    parser.add_argument("--response-format", choices=RESPONSE_FORMAT_CHOICES)
    parser.add_argument("--output-format", choices=OUTPUT_FORMAT_CHOICES)
    parser.add_argument("--output", help="Downloaded image path.")
    parser.add_argument("--request-timeout", type=float)
    parser.add_argument("--dry-run", action="store_true", help="Print the request shape without sending it.")
    parser.set_defaults(dry_run=None)
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
        submit_url = build_submit_url(base_url, endpoint)
        payload = make_request_payload(args)
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers = {
            "Authorization": f"Bearer {api_key}",
            "Accept": "application/json",
            "Content-Type": "application/json",
            "User-Agent": "hermestoken-gpt-image2-python/1.0",
        }

        if args.dry_run:
            print(f"POST {submit_url}", file=stdout)
            print(f"Authorization: Bearer {redact_secret(api_key)}", file=stdout)
            print("Content-Type: application/json", file=stdout)
            print("payload:", file=stdout)
            print(json.dumps(payload, ensure_ascii=False, indent=2), file=stdout)
            print(f"output={args.output}", file=stdout)
            return 0

        started = time.monotonic()
        response_payload = http_request_json(
            submit_url,
            "POST",
            headers,
            body,
            timeout=args.request_timeout,
        )
        print(json.dumps(response_payload, ensure_ascii=False, indent=2), file=stdout)
        saved_paths = save_images(
            response_payload=response_payload,
            base_url=base_url,
            api_key=api_key,
            output_path=args.output,
            timeout=args.request_timeout,
        )
        for path in saved_paths:
            print(f"[ok] saved={path}", file=stdout)
        print(f"[ok] elapsed={time.monotonic() - started:.2f}s", file=stdout)
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
