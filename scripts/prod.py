import argparse
import hashlib
import os
import shutil
import sys
import tempfile
from pathlib import Path
from typing import Mapping, Optional, TextIO
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen

from cloudflare_static import (
    DEFAULT_COMPATIBILITY_DATE,
    deploy_static_assets as deploy_cloudflare_static_assets,
)
from launcher_common import (
    FRONTEND_DIST_ASSET_REFERENCE_PATTERN,
    LauncherError,
    normalize_asset_base_url,
    poll_http_until_healthy,
    prepare_frontend_dist_for_docker_packaging,
    print_actionable_error,
    remove_legacy_compose_containers,
    require_docker_and_compose,
    resolve_application_version,
    resolve_web_dist_strategy,
    run_command,
    validate_frontend_dist_integrity,
)


REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_PROD_COMPOSE_FILE = "docker-compose.prod.yml"
DEFAULT_PROD_ENV_FILE = ".env.production"
DEFAULT_HEALTHCHECK_TIMEOUT_SECONDS = 60
DEFAULT_HEALTHCHECK_INTERVAL_SECONDS = 1
DEFAULT_PUBLIC_FRONTEND_TIMEOUT_SECONDS = 15
PROD_CONTAINER_NAMES = ("hermestoken-prod", "hermestoken-prod-postgres", "hermestoken-prod-redis")
DEFAULT_NGINX_SITE_PATH = Path("/etc/nginx/sites-available/default")
DEFAULT_NGINX_CONF_D_PATH = Path("/etc/nginx/conf.d")
NGINX_BACKEND_PROXY_PREFIXES = (
    "/api/",
    "/dashboard/",
    "/v1/",
    "/v1beta/",
    "/pg/",
    "/mj/",
    "/suno/",
    "/kling/",
)
NGINX_BACKEND_PROXY_REGEXES = (
    r"^/[^/]+/mj/",
)
NGINX_COMPRESSIBLE_TYPES = """        text/plain
        text/css
        text/javascript
        application/javascript
        application/json
        application/manifest+json
        application/xml
        image/svg+xml;
"""

CLOUDFLARE_REAL_IP_CIDRS = (
    "173.245.48.0/20",
    "103.21.244.0/22",
    "103.22.200.0/22",
    "103.31.4.0/22",
    "141.101.64.0/18",
    "108.162.192.0/18",
    "190.93.240.0/20",
    "188.114.96.0/20",
    "197.234.240.0/22",
    "198.41.128.0/17",
    "162.158.0.0/15",
    "104.16.0.0/13",
    "104.24.0.0/14",
    "172.64.0.0/13",
    "131.0.72.0/22",
    "2400:cb00::/32",
    "2606:4700::/32",
    "2803:f800::/32",
    "2405:b500::/32",
    "2405:8100::/32",
    "2a06:98c0::/29",
    "2c0f:f248::/32",
)


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


def _public_url_for_path(public_url: str, asset_path: str) -> str:
    return f"{public_url.rstrip('/')}/{asset_path.lstrip('/')}"


def _sha256_bytes(content: bytes) -> str:
    return hashlib.sha256(content).hexdigest()


def _download_public_asset(url: str, *, timeout_seconds: int) -> bytes:
    request = Request(url, headers={"User-Agent": "hermestoken-prod-launcher/1.0"})
    try:
        with urlopen(request, timeout=timeout_seconds) as response:
            return response.read()
    except (HTTPError, URLError, TimeoutError, OSError) as exc:
        raise LauncherError(
            f"Failed to download public frontend asset: {url}. Next step: verify nginx points to the freshly built frontend dist and rerun `python3 scripts/prod.py update --domain <public-url>`."
        ) from exc


def verify_public_frontend_dist(
    *,
    public_url: str,
    dist_dir: Path,
    output: Optional[TextIO] = None,
    timeout_seconds: int = DEFAULT_PUBLIC_FRONTEND_TIMEOUT_SECONDS,
) -> None:
    stream = output or sys.stdout
    validate_frontend_dist_integrity(dist_dir)

    index_path = dist_dir / "index.html"
    local_index = index_path.read_bytes()
    public_index = _download_public_asset(
        _public_url_for_path(public_url, "/index.html"),
        timeout_seconds=timeout_seconds,
    )
    if _sha256_bytes(public_index) != _sha256_bytes(local_index):
        raise LauncherError(
            "Public frontend verification failed: `/index.html` does not match the freshly built frontend `index.html`. Next step: ensure nginx serves the host dist generated by the launcher, then rerun the production script."
        )

    index_html = index_path.read_text(encoding="utf-8")
    asset_paths = sorted(
        {
            match.group(1).split("?", 1)[0].split("#", 1)[0]
            for match in FRONTEND_DIST_ASSET_REFERENCE_PATTERN.finditer(index_html)
        }
    )
    for asset_path in asset_paths:
        local_asset_path = dist_dir / asset_path.lstrip("/")
        public_asset = _download_public_asset(
            _public_url_for_path(public_url, asset_path),
            timeout_seconds=timeout_seconds,
        )
        if _sha256_bytes(public_asset) != _sha256_bytes(local_asset_path.read_bytes()):
            raise LauncherError(
                f"Public frontend verification failed: `{asset_path}` does not match the freshly built host dist asset. Next step: update the host frontend dist through `python3 scripts/prod.py update --domain {public_url.rstrip('/')}` instead of running raw docker compose."
            )

    stream.write(
        f"[ok] Public frontend assets match host dist: {public_url.rstrip('/')} ({len(asset_paths)} assets)\n"
    )


def build_nginx_site_config(
    *,
    public_url: str,
    app_port: str = "3000",
    include_real_ip_directives: bool = True,
    frontend_dist_path: Optional[Path] = None,
    enable_brotli: bool = False,
) -> str:
    parsed = urlparse(public_url)
    hostname = (parsed.hostname or "").strip()
    if not hostname:
        raise LauncherError(
            "Public URL must include a hostname before generating the nginx config. Next step: rerun with `--domain https://hermestoken.top`."
        )

    canonical_host = hostname[4:] if hostname.startswith("www.") else hostname
    www_host = f"www.{canonical_host}"

    real_ip_lines = "\n".join(f"set_real_ip_from {cidr};" for cidr in CLOUDFLARE_REAL_IP_CIDRS)

    real_ip_block = ""
    if include_real_ip_directives:
        real_ip_block = f"""# If the site is proxied by Cloudflare, enable real client IP restoration.
# Source of CIDRs:
# https://www.cloudflare.com/ips-v4
# https://www.cloudflare.com/ips-v6
real_ip_header CF-Connecting-IP;
real_ip_recursive on;

{real_ip_lines}

"""

    proxy_header_directives = """        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 3600;
        proxy_send_timeout 3600;
        proxy_buffering off;

        add_header X-Nginx-Request-Time $request_time always;
        add_header X-Nginx-Upstream-Response-Time $upstream_response_time always;
        add_header X-Nginx-Upstream-Connect-Time $upstream_connect_time always;
"""

    backend_proxy_directives = (
        f"        proxy_pass http://127.0.0.1:{app_port};\n{proxy_header_directives}"
    )
    public_home_proxy_directives = (
        f"        proxy_pass http://127.0.0.1:{app_port}/__internal/public-home;\n"
        f"{proxy_header_directives}"
        '        add_header Cache-Control "no-cache" always;\n'
    )

    static_assets_block = ""
    if frontend_dist_path is not None:
        dist_root = Path(frontend_dist_path).expanduser().as_posix()
        proxy_location_blocks = "\n\n".join(
            f"""    location ^~ {prefix} {{
{backend_proxy_directives}    }}"""
            for prefix in NGINX_BACKEND_PROXY_PREFIXES
        )
        proxy_regex_blocks = "\n\n".join(
            f"""    location ~ {pattern} {{
{backend_proxy_directives}    }}"""
            for pattern in NGINX_BACKEND_PROXY_REGEXES
        )
        static_assets_block = f"""    location ^~ /assets/ {{
        root {dist_root};
        access_log off;
        etag on;
        try_files $uri =404;
        add_header Access-Control-Allow-Origin "*" always;
        add_header Cache-Control "public, max-age=31536000, immutable" always;
    }}

    location = / {{
{public_home_proxy_directives}    }}

    location ^~ /jimeng {{
{backend_proxy_directives}    }}

{proxy_location_blocks}

{proxy_regex_blocks}

    location = /index.html {{
        root {dist_root};
        etag on;
        try_files $uri =404;
        add_header Cache-Control "no-cache" always;
    }}

    location / {{
        root {dist_root};
        index index.html;
        etag on;
        try_files $uri $uri/ /index.html;
    }}
"""
    else:
        static_assets_block = f"""    location / {{
{backend_proxy_directives}    }}
"""

    brotli_block = ""
    if enable_brotli:
        brotli_block = """
    brotli on;
    brotli_comp_level 5;
    brotli_static on;
    brotli_types
""" + NGINX_COMPRESSIBLE_TYPES.rstrip()

    return f"""map $http_upgrade $connection_upgrade {{
    default upgrade;
    '' close;
}}

{real_ip_block}\
server {{
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name {canonical_host} {www_host} _;

    return 301 https://{canonical_host}$request_uri;
}}

server {{
    listen 443 ssl http2 default_server;
    listen [::]:443 ssl http2 default_server;
    server_name {www_host} _;

    ssl_certificate /etc/letsencrypt/live/{canonical_host}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/{canonical_host}/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    return 301 https://{canonical_host}$request_uri;
}}

server {{
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name {canonical_host};

    ssl_certificate /etc/letsencrypt/live/{canonical_host}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/{canonical_host}/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    client_max_body_size 100m;
    gzip on;
    gzip_comp_level 5;
    gzip_min_length 1024;
    gzip_proxied any;
    gzip_vary on;
    gzip_types
{NGINX_COMPRESSIBLE_TYPES.rstrip()}{brotli_block}

{static_assets_block}\
}}
"""


def detect_nginx_supports_brotli() -> bool:
    with tempfile.TemporaryDirectory(prefix="nginx-brotli-probe-") as tmp_dir:
        tmp_path = Path(tmp_dir)
        config_path = tmp_path / "nginx.conf"
        config_path.write_text(
            f"""pid {tmp_path / "nginx.pid"};
error_log stderr notice;

events {{}}

http {{
    brotli on;
    brotli_comp_level 5;
    brotli_static on;
    brotli_types
{NGINX_COMPRESSIBLE_TYPES.rstrip()}
}}
""",
            encoding="utf-8",
        )

        try:
            completed = run_command(
                ["nginx", "-t", "-p", tmp_path.as_posix(), "-c", str(config_path)],
                check=False,
                stream_output=False,
            )
        except LauncherError:
            return False

    return completed.returncode == 0


def detect_real_ip_conf_in_conf_d(*, conf_d_path: Path = DEFAULT_NGINX_CONF_D_PATH) -> bool:
    if not conf_d_path.is_dir():
        return False

    for conf_path in sorted(conf_d_path.glob("*.conf")):
        try:
            content = conf_path.read_text(encoding="utf-8")
        except OSError:
            continue

        if "real_ip_header" in content or "set_real_ip_from" in content:
            return True

    return False


def sync_nginx_site_config(
    *,
    public_url: str,
    env_values: Mapping[str, str],
    output: Optional[TextIO] = None,
    site_path: Optional[Path] = None,
    conf_d_path: Optional[Path] = None,
    frontend_dist_path: Optional[Path] = None,
) -> bool:
    stream = output or sys.stdout

    if shutil.which("nginx") is None:
        stream.write("[info] nginx not found; skipped nginx site config sync\n")
        return False

    target_path = site_path or DEFAULT_NGINX_SITE_PATH
    effective_conf_d_path = conf_d_path or DEFAULT_NGINX_CONF_D_PATH
    app_port = env_values.get("APP_PORT", "3000").strip()
    if not app_port.isdigit():
        app_port = "3000"

    include_real_ip_directives = not detect_real_ip_conf_in_conf_d(
        conf_d_path=effective_conf_d_path
    )
    if not include_real_ip_directives:
        stream.write(
            f"[info] Detected existing real_ip nginx config in {effective_conf_d_path}; "
            "site config will not duplicate Cloudflare real IP directives\n"
        )

    rendered = build_nginx_site_config(
        public_url=public_url,
        app_port=app_port,
        include_real_ip_directives=include_real_ip_directives,
        frontend_dist_path=frontend_dist_path,
        enable_brotli=detect_nginx_supports_brotli(),
    )
    current = target_path.read_text(encoding="utf-8") if target_path.exists() else None
    if current == rendered:
        stream.write(f"[ok] Nginx site config already up to date: {target_path}\n")
    else:
        try:
            target_path.parent.mkdir(parents=True, exist_ok=True)
            target_path.write_text(rendered, encoding="utf-8")
        except PermissionError as exc:
            raise LauncherError(
                f"Failed to write nginx site config: {target_path}. Next step: rerun the production script with sudo/root so it can update nginx."
            ) from exc
        stream.write(f"[ok] Nginx site config synced: {target_path}\n")

    run_command(["nginx", "-t"], check=True, stream_output=False)
    run_command(["systemctl", "reload", "nginx"], check=True, stream_output=False)
    stream.write("[ok] Nginx reloaded with updated site config\n")
    return True


def run_stack(
    *,
    action_label: str,
    compose_file_path: Path,
    env_file_path: Path,
    local_health_url: str,
    output: Optional[TextIO] = None,
    repo_root: Optional[Path] = None,
    asset_base_url: str = "",
    cloudflare_worker_name: str = "",
) -> None:
    stream = output or sys.stdout
    effective_repo_root = repo_root or REPO_ROOT
    web_dist_strategy = resolve_web_dist_strategy()
    app_version = resolve_application_version(repo_root=effective_repo_root)

    require_docker_and_compose()
    stream.write("[ok] Docker available\n")
    frontend_build_env = None
    if asset_base_url:
        frontend_build_env = {
            "VITE_ASSET_BASE_URL": normalize_asset_base_url(asset_base_url),
        }

    if frontend_build_env is None:
        prepare_frontend_dist_for_docker_packaging(
            output=stream,
            repo_root=effective_repo_root,
        )
    else:
        prepare_frontend_dist_for_docker_packaging(
            output=stream,
            repo_root=effective_repo_root,
            env=frontend_build_env,
        )

    if cloudflare_worker_name:
        deploy_cloudflare_static_assets(
            worker_name=cloudflare_worker_name,
            asset_dir=effective_repo_root / "web" / "classic" / "dist",
            env=os.environ,
            output=stream,
            repo_root=effective_repo_root,
            compatibility_date=DEFAULT_COMPATIBILITY_DATE,
            message=f"{action_label} {app_version}",
        )

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
            "hermestoken",
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
        _compose_command_prefix(compose_file_path, env_file_path) + ["restart", "hermestoken"],
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
        command.add_argument("--asset-base-url", default="")
        command.add_argument("--cloudflare-worker-name", default="")

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
            asset_base_url=normalize_asset_base_url(args.asset_base_url)
            if args.asset_base_url
            else "",
            cloudflare_worker_name=(args.cloudflare_worker_name or "").strip(),
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
            sync_nginx_site_config(
                public_url=args.domain,
                env_values=env_values,
                output=sys.stdout,
            )

        return 0
    except LauncherError as exc:
        print_actionable_error(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
