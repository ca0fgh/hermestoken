import argparse
import sys
from pathlib import Path
from typing import Optional, TextIO

from launcher_common import (
    LauncherConfig,
    LauncherError,
    ensure_named_docker_volume,
    load_launcher_config,
    poll_http_until_healthy,
    print_actionable_error,
    remove_legacy_compose_containers,
    prepare_frontend_dist_for_docker_packaging,
    require_docker_and_compose,
    resolve_application_version,
    resolve_web_dist_strategy,
    run_browser_smoke_check,
    run_command,
)


REPO_ROOT = Path(__file__).resolve().parents[1]
LOCAL_CONTAINER_NAMES = ("new-api", "postgres", "redis")
LOCAL_NAMED_VOLUMES = {"pg_data": "hermestoken_pg_data"}


def _compose_file_path(compose_file: str, *, repo_root: Path) -> Path:
    compose_path = Path(compose_file).expanduser()
    if compose_path.is_absolute():
        return compose_path
    return repo_root / compose_path


def _print_recent_container_status(compose_file_path: Path, *, output: TextIO, repo_root: Path) -> None:
    output.write("[info] Recent container status (docker compose ps):\n")
    try:
        status_result = run_command(
            ["docker", "compose", "-f", str(compose_file_path), "ps"],
            check=False,
            stream_output=False,
            cwd=repo_root,
        )
    except LauncherError as exc:
        output.write(f"[info] Unable to fetch container status: {exc}\n")
        return

    if status_result.returncode != 0:
        details = (status_result.stderr or status_result.stdout or "").strip()
        if details:
            output.write(f"[info] `docker compose ps` failed ({status_result.returncode}): {details}\n")
            return
        output.write(f"[info] `docker compose ps` failed with exit code {status_result.returncode}.\n")
        return

    status_output = (status_result.stdout or "").strip()
    if status_output:
        output.write(f"{status_output}\n")
        return
    output.write("[info] No container status output was returned.\n")


def run_local_stack(
    config: LauncherConfig,
    *,
    output: Optional[TextIO] = None,
    repo_root: Optional[Path] = None,
    action_label: str = "deploy",
) -> None:
    stream = output or sys.stdout
    effective_repo_root = repo_root or REPO_ROOT
    compose_file_path = _compose_file_path(config.compose_file, repo_root=effective_repo_root)
    web_dist_strategy = resolve_web_dist_strategy()
    app_version = resolve_application_version(repo_root=effective_repo_root)
    require_docker_and_compose()
    stream.write("[ok] Docker available\n")
    prepare_frontend_dist_for_docker_packaging(output=stream, repo_root=effective_repo_root)

    remove_legacy_compose_containers(
        legacy_project_name="hermestoken",
        compose_file_path=compose_file_path,
        container_names=LOCAL_CONTAINER_NAMES,
        output=stream,
        repo_root=effective_repo_root,
    )
    ensure_named_docker_volume(
        LOCAL_NAMED_VOLUMES["pg_data"],
        output=stream,
        repo_root=effective_repo_root,
    )
    run_command(
        ["docker", "compose", "-f", str(compose_file_path), "up", "-d", "--build"],
        check=True,
        stream_output=True,
        cwd=effective_repo_root,
        env={"WEB_DIST_STRATEGY": web_dist_strategy, "APP_VERSION": app_version},
        stdout_stream=stream,
    )
    stream.write("[ok] Containers started\n")

    try:
        poll_http_until_healthy(
            config.local_url,
            timeout_seconds=config.healthcheck_timeout_seconds,
            interval_seconds=config.healthcheck_interval_seconds,
        )
        run_browser_smoke_check(config.local_url, output=stream)
    except LauncherError:
        _print_recent_container_status(compose_file_path, output=stream, repo_root=effective_repo_root)
        raise
    stream.write(f"[ok] Local {action_label} healthy: {config.local_url}\n")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Deploy or update the local Docker stack for HERMESTOKEN.")
    parser.add_argument("command", nargs="?", choices=("deploy", "update"), default="deploy")
    return parser


def main() -> int:
    try:
        args = build_parser().parse_args()
        config = load_launcher_config()
        run_local_stack(config, output=sys.stdout, action_label=args.command)
        return 0
    except LauncherError as exc:
        print_actionable_error(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
