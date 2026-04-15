import argparse
import sys
from pathlib import Path
from typing import Mapping, Optional, TextIO
from urllib.parse import urlparse

from launcher_common import (
    LauncherError,
    poll_http_until_healthy,
    prepare_frontend_dist_for_docker_packaging,
    print_actionable_error,
    remove_legacy_compose_containers,
    require_docker_and_compose,
    resolve_application_version,
    resolve_web_dist_strategy,
    run_command,
)


REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_PROD_COMPOSE_FILE = "docker-compose.prod.yml"
DEFAULT_PROD_ENV_FILE = ".env.production"
DEFAULT_HEALTHCHECK_TIMEOUT_SECONDS = 60
DEFAULT_HEALTHCHECK_INTERVAL_SECONDS = 1
PROD_CONTAINER_NAMES = ("hermestoken-prod", "hermestoken-prod-postgres", "hermestoken-prod-redis")


def _resolve_repo_path(path_value: str, *, repo_root: Path) -> Path:
    path = Path(path_value).expanduser()
    if path.is_absolute():
        return path
    return repo_root / path


def load_env_file(env_file_path: Path) -> dict[str, str]:
    try:
        raw = env_file_path.read_text(encoding="utf-8")
    except FileNotFoundError as exc:
        raise LauncherError(
            f"Production env file not found: {env_file_path} Next step: Copy `.env.production.example` to `.env.production` and fill in the secrets."
        ) from exc

    values: dict[str, str] = {}
    for line in raw.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or "=" not in stripped:
            continue
        key, value = stripped.split("=", 1)
        values[key.strip()] = value.strip()
    return values


def build_local_health_url(env_values: Mapping[str, str]) -> str:
    app_port = env_values.get("APP_PORT", "3000").strip()
    if not app_port.isdigit():
        app_port = "3000"
    return f"http://127.0.0.1:{app_port}/api/status"


def _compose_command_prefix(compose_file_path: Path, env_file_path: Path) -> list[str]:
    return [
        "docker",
        "compose",
        "--env-file",
        str(env_file_path),
        "-f",
        str(compose_file_path),
    ]


def run_stack(
    *,
    action_label: str,
    compose_file_path: Path,
    env_file_path: Path,
    local_health_url: str,
    output: Optional[TextIO] = None,
    repo_root: Optional[Path] = None,
) -> None:
    stream = output or sys.stdout
    effective_repo_root = repo_root or REPO_ROOT
    web_dist_strategy = resolve_web_dist_strategy()
    app_version = resolve_application_version(repo_root=effective_repo_root)

    require_docker_and_compose()
    stream.write("[ok] Docker available\n")
    prepare_frontend_dist_for_docker_packaging(output=stream, repo_root=effective_repo_root)

    remove_legacy_compose_containers(
        legacy_project_name="hermestoken",
        compose_file_path=compose_file_path,
        container_names=PROD_CONTAINER_NAMES,
        output=stream,
        repo_root=effective_repo_root,
    )

    run_command(
        _compose_command_prefix(compose_file_path, env_file_path) + ["up", "-d", "--build"],
        check=True,
        stream_output=True,
        cwd=effective_repo_root,
        env={"WEB_DIST_STRATEGY": web_dist_strategy, "APP_VERSION": app_version},
        stdout_stream=stream,
    )
    stream.write(f"[ok] Production {action_label} containers started\n")

    poll_http_until_healthy(
        local_health_url,
        timeout_seconds=DEFAULT_HEALTHCHECK_TIMEOUT_SECONDS,
        interval_seconds=DEFAULT_HEALTHCHECK_INTERVAL_SECONDS,
    )
    stream.write(f"[ok] Production {action_label} healthy: {local_health_url}\n")


def _sql_literal(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def set_public_url(
    *,
    compose_file_path: Path,
    env_file_path: Path,
    public_url: str,
    local_health_url: str,
    output: Optional[TextIO] = None,
    repo_root: Optional[Path] = None,
) -> None:
    stream = output or sys.stdout
    effective_repo_root = repo_root or REPO_ROOT

    parsed = urlparse(public_url)
    if not parsed.scheme or not parsed.hostname:
        raise LauncherError(
            "Public URL must be a full URL such as `https://hermestoken.top`. Next step: rerun with `--domain https://hermestoken.top`."
        )

    rp_id = parsed.hostname
    origins = public_url.rstrip("/")
    app_url = public_url.rstrip("/")

    sql = (
        "BEGIN; "
        f"UPDATE options SET value = {_sql_literal(app_url)} WHERE key = 'ServerAddress'; "
        f"INSERT INTO options (key, value) SELECT 'ServerAddress', {_sql_literal(app_url)} "
        "WHERE NOT EXISTS (SELECT 1 FROM options WHERE key = 'ServerAddress'); "
        f"UPDATE options SET value = {_sql_literal(rp_id)} WHERE key = 'passkey.rp_id'; "
        f"INSERT INTO options (key, value) SELECT 'passkey.rp_id', {_sql_literal(rp_id)} "
        "WHERE NOT EXISTS (SELECT 1 FROM options WHERE key = 'passkey.rp_id'); "
        f"UPDATE options SET value = {_sql_literal(origins)} WHERE key = 'passkey.origins'; "
        f"INSERT INTO options (key, value) SELECT 'passkey.origins', {_sql_literal(origins)} "
        "WHERE NOT EXISTS (SELECT 1 FROM options WHERE key = 'passkey.origins'); "
        "COMMIT;"
    )

    run_command(
        _compose_command_prefix(compose_file_path, env_file_path)
        + [
            "exec",
            "-T",
            "postgres",
            "psql",
            "-U",
            "root",
            "-d",
            "new-api",
            "-v",
            "ON_ERROR_STOP=1",
            "-c",
            sql,
        ],
        check=True,
        stream_output=False,
        cwd=effective_repo_root,
    )

    run_command(
        _compose_command_prefix(compose_file_path, env_file_path) + ["restart", "new-api"],
        check=True,
        stream_output=True,
        cwd=effective_repo_root,
        stdout_stream=stream,
    )

    poll_http_until_healthy(
        local_health_url,
        timeout_seconds=DEFAULT_HEALTHCHECK_TIMEOUT_SECONDS,
        interval_seconds=DEFAULT_HEALTHCHECK_INTERVAL_SECONDS,
    )
    stream.write(f"Updated ServerAddress to: {app_url}\n")
    stream.write(f"Updated passkey.rp_id to: {rp_id}\n")
    stream.write(f"Updated passkey.origins to: {origins}\n")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Deploy or update the production stack for HERMESTOKEN.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    for command_name in ("deploy", "update"):
        command = subparsers.add_parser(command_name)
        command.add_argument("--compose-file", default=DEFAULT_PROD_COMPOSE_FILE)
        command.add_argument("--env-file", default=DEFAULT_PROD_ENV_FILE)
        command.add_argument("--domain", default="")

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    try:
        compose_file_path = _resolve_repo_path(args.compose_file, repo_root=REPO_ROOT)
        env_file_path = _resolve_repo_path(args.env_file, repo_root=REPO_ROOT)
        env_values = load_env_file(env_file_path)
        local_health_url = build_local_health_url(env_values)

        run_stack(
            action_label=args.command,
            compose_file_path=compose_file_path,
            env_file_path=env_file_path,
            local_health_url=local_health_url,
            output=sys.stdout,
            repo_root=REPO_ROOT,
        )

        if args.domain:
            set_public_url(
                compose_file_path=compose_file_path,
                env_file_path=env_file_path,
                public_url=args.domain,
                local_health_url=local_health_url,
                output=sys.stdout,
                repo_root=REPO_ROOT,
            )

        return 0
    except LauncherError as exc:
        print_actionable_error(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
