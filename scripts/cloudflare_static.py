import argparse
import os
import sys
from datetime import date
from pathlib import Path
from typing import Mapping, Optional, TextIO

from launcher_common import (
    LauncherError,
    print_actionable_error,
    require_executable,
    run_command,
)


REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_COMPATIBILITY_DATE = date.today().isoformat()


def _require_cloudflare_env(env: Mapping[str, str], key: str) -> str:
    value = (env.get(key) or "").strip()
    if value:
        return value
    raise LauncherError(
        f"Missing required Cloudflare credential: {key}. Next step: export {key} before deploying static assets."
    )


def deploy_static_assets(
    *,
    worker_name: str,
    asset_dir: Path,
    env: Mapping[str, str],
    output: Optional[TextIO] = None,
    repo_root: Optional[Path] = None,
    compatibility_date: str = DEFAULT_COMPATIBILITY_DATE,
    message: str = "",
) -> None:
    stream = output or sys.stdout
    effective_repo_root = repo_root or REPO_ROOT

    if not worker_name.strip():
        raise LauncherError(
            "Missing Cloudflare worker name. Next step: rerun with --worker-name <worker-name>."
        )
    if not asset_dir.is_dir():
        raise LauncherError(
            f"Missing static asset directory: {asset_dir}. Next step: build the frontend so the asset directory exists before deploying."
        )

    require_executable(
        "bunx",
        install_hint="Install Bun and verify `bunx --version` succeeds before deploying Cloudflare static assets.",
    )

    account_id = _require_cloudflare_env(env, "CLOUDFLARE_ACCOUNT_ID")
    api_token = _require_cloudflare_env(env, "CLOUDFLARE_API_TOKEN")

    command = [
        "bunx",
        "wrangler",
        "deploy",
        "--name",
        worker_name.strip(),
        "--compatibility-date",
        compatibility_date,
        "--assets",
        str(asset_dir),
    ]
    if message.strip():
        command.extend(["--message", message.strip()])

    run_command(
        command,
        check=True,
        stream_output=True,
        cwd=effective_repo_root,
        env={
            "CLOUDFLARE_ACCOUNT_ID": account_id,
            "CLOUDFLARE_API_TOKEN": api_token,
            "CI": "1",
        },
        stdout_stream=stream,
    )
    stream.write(
        f"[ok] Cloudflare static assets deployed: worker={worker_name.strip()} assets={asset_dir}\n"
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Deploy static frontend assets to a Cloudflare Worker static-assets project."
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    deploy = subparsers.add_parser("deploy")
    deploy.add_argument("--worker-name", required=True)
    deploy.add_argument("--asset-dir", default="web/dist")
    deploy.add_argument("--compatibility-date", default=DEFAULT_COMPATIBILITY_DATE)
    deploy.add_argument("--message", default="")

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    try:
        if args.command == "deploy":
            asset_dir = Path(args.asset_dir).expanduser()
            if not asset_dir.is_absolute():
                asset_dir = REPO_ROOT / asset_dir

            deploy_static_assets(
                worker_name=args.worker_name,
                asset_dir=asset_dir,
                env=os.environ,
                output=sys.stdout,
                repo_root=REPO_ROOT,
                compatibility_date=args.compatibility_date,
                message=args.message,
            )
        return 0
    except LauncherError as exc:
        print_actionable_error(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
