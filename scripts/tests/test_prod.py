import io
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import launcher_common
import prod


class ProdLauncherTests(unittest.TestCase):
    def test_prod_compose_uses_parent_postgres_volume_for_latest_image(self):
        repo_root = Path(__file__).resolve().parents[2]
        compose_file = repo_root / "docker-compose.prod.yml"
        compose_text = compose_file.read_text(encoding="utf-8")

        self.assertRegex(compose_text, r"(?m)^name:\s+hermestoken-prod$")
        self.assertIn("name: hermestoken_pg_data_prod", compose_text)
        self.assertIn("image: postgres:latest", compose_text)
        self.assertIn("- pg_data_prod:/var/lib/postgresql", compose_text)
        self.assertNotIn("- pg_data_prod:/var/lib/postgresql/data", compose_text)

    def test_prod_compose_defaults_to_prebuilt_frontend_packaging(self):
        repo_root = Path(__file__).resolve().parents[2]
        compose_text = (repo_root / "docker-compose.prod.yml").read_text(encoding="utf-8")

        self.assertIn("WEB_DIST_STRATEGY: ${WEB_DIST_STRATEGY:-prebuilt}", compose_text)
        self.assertIn("APP_VERSION: ${APP_VERSION:-}", compose_text)

    def test_build_local_health_url_uses_port_from_env_file(self):
        self.assertEqual(
            prod.build_local_health_url({"APP_PORT": "4567"}),
            "http://127.0.0.1:4567/api/status",
        )

    def test_build_local_health_url_falls_back_to_default_port(self):
        self.assertEqual(
            prod.build_local_health_url({}),
            "http://127.0.0.1:3000/api/status",
        )
        self.assertEqual(
            prod.build_local_health_url({"APP_PORT": "not-a-number"}),
            "http://127.0.0.1:3000/api/status",
        )

    def test_build_nginx_site_config_uses_domain_port_and_cloudflare_real_ip(self):
        config = prod.build_nginx_site_config(
            public_url="https://hermestoken.top",
            app_port="4567",
        )

        self.assertIn("real_ip_header CF-Connecting-IP;", config)
        self.assertIn("set_real_ip_from 173.245.48.0/20;", config)
        self.assertIn("set_real_ip_from 2c0f:f248::/32;", config)
        self.assertIn("server_name hermestoken.top www.hermestoken.top _;", config)
        self.assertIn("server_name www.hermestoken.top _;", config)
        self.assertIn("server_name hermestoken.top;", config)
        self.assertIn("listen 443 ssl http2 default_server;", config)
        self.assertIn("listen [::]:443 ssl http2 default_server;", config)
        self.assertIn("listen 443 ssl http2;", config)
        self.assertIn("listen [::]:443 ssl http2;", config)
        self.assertIn("proxy_pass http://127.0.0.1:4567;", config)
        self.assertIn("gzip on;", config)
        self.assertIn("gzip_comp_level 5;", config)
        self.assertIn("application/javascript", config)
        self.assertIn("application/json", config)

    def test_build_nginx_site_config_can_skip_cloudflare_real_ip_block(self):
        config = prod.build_nginx_site_config(
            public_url="https://hermestoken.top",
            app_port="3000",
            include_real_ip_directives=False,
        )

        self.assertNotIn("real_ip_header CF-Connecting-IP;", config)
        self.assertNotIn("set_real_ip_from 173.245.48.0/20;", config)
        self.assertIn("proxy_pass http://127.0.0.1:3000;", config)

    def test_build_nginx_site_config_can_serve_fingerprinted_assets_directly_from_host_dist(self):
        config = prod.build_nginx_site_config(
            public_url="https://hermestoken.top",
            app_port="3000",
            frontend_dist_path=Path("/opt/hermestoken/web/dist"),
        )

        self.assertIn("location ^~ /assets/ {", config)
        self.assertIn("root /opt/hermestoken/web/dist;", config)
        self.assertIn('add_header Cache-Control "public, max-age=31536000, immutable" always;', config)
        self.assertIn("try_files $uri =404;", config)

    def test_build_nginx_site_config_can_serve_spa_routes_from_host_dist_and_proxy_backend_prefixes(self):
        config = prod.build_nginx_site_config(
            public_url="https://hermestoken.top",
            app_port="3000",
            frontend_dist_path=Path("/opt/hermestoken/web/dist"),
        )

        self.assertIn("location = /index.html {", config)
        self.assertIn('add_header Cache-Control "no-cache" always;', config)
        self.assertIn("location / {", config)
        self.assertIn("try_files $uri $uri/ /index.html;", config)
        self.assertIn("location ^~ /api/ {", config)
        self.assertIn("location ^~ /v1/ {", config)
        self.assertIn("location ^~ /v1beta/ {", config)
        self.assertIn("location ^~ /pg/ {", config)
        self.assertIn("location ^~ /mj/ {", config)
        self.assertIn("location ^~ /suno/ {", config)
        self.assertIn("location ^~ /kling/ {", config)
        self.assertIn("location ^~ /jimeng", config)
        self.assertIn("location ^~ /dashboard/ {", config)
        self.assertIn("location ~ ^/[^/]+/mj/ {", config)
        self.assertNotIn("location / {\n        proxy_pass", config)

    def test_detect_real_ip_conf_returns_true_when_conf_d_already_manages_directives(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            conf_d_path = Path(tmp_dir) / "conf.d"
            conf_d_path.mkdir(parents=True, exist_ok=True)
            (conf_d_path / "cloudflare-real-ip.conf").write_text(
                "real_ip_header CF-Connecting-IP;\nset_real_ip_from 173.245.48.0/20;\n",
                encoding="utf-8",
            )

            self.assertTrue(
                prod.detect_real_ip_conf_in_conf_d(conf_d_path=conf_d_path)
            )

    @mock.patch("prod.run_command")
    @mock.patch("prod.shutil.which", return_value="/usr/sbin/nginx")
    def test_sync_nginx_site_config_avoids_duplicate_real_ip_block_when_conf_d_exists(
        self,
        which,
        run_command,
    ):
        stdout = io.StringIO()
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            site_path = tmp_path / "sites-available" / "default"
            conf_d_path = tmp_path / "conf.d"
            conf_d_path.mkdir(parents=True, exist_ok=True)
            conf_d_file = conf_d_path / "cloudflare-real-ip.conf"
            conf_d_file.write_text(
                "real_ip_header CF-Connecting-IP;\nset_real_ip_from 173.245.48.0/20;\n",
                encoding="utf-8",
            )

            changed = prod.sync_nginx_site_config(
                public_url="https://hermestoken.top",
                env_values={"APP_PORT": "3000"},
                output=stdout,
                site_path=site_path,
                conf_d_path=conf_d_path,
            )

            rendered = site_path.read_text(encoding="utf-8")

        self.assertTrue(changed)
        which.assert_called_once_with("nginx")
        self.assertNotIn("real_ip_header CF-Connecting-IP;", rendered)
        self.assertIn("proxy_pass http://127.0.0.1:3000;", rendered)
        self.assertIn("existing real_ip nginx config", stdout.getvalue())

    @mock.patch("prod.run_command")
    @mock.patch("prod.shutil.which", return_value="/usr/sbin/nginx")
    def test_sync_nginx_site_config_writes_file_and_reloads_nginx(
        self,
        which,
        run_command,
    ):
        stdout = io.StringIO()
        with tempfile.TemporaryDirectory() as tmp_dir:
            site_path = Path(tmp_dir) / "default"
            changed = prod.sync_nginx_site_config(
                public_url="https://hermestoken.top",
                env_values={"APP_PORT": "3000"},
                output=stdout,
                site_path=site_path,
            )

        self.assertTrue(changed)
        which.assert_called_once_with("nginx")
        self.assertEqual(run_command.call_count, 2)
        self.assertEqual(
            run_command.call_args_list[0],
            mock.call(["nginx", "-t"], check=True, stream_output=False),
        )
        self.assertEqual(
            run_command.call_args_list[1],
            mock.call(["systemctl", "reload", "nginx"], check=True, stream_output=False),
        )
        self.assertIn("Nginx site config synced", stdout.getvalue())
        self.assertIn("Nginx reloaded", stdout.getvalue())

    @mock.patch("prod.run_command")
    @mock.patch("prod.shutil.which", return_value="/usr/sbin/nginx")
    def test_sync_nginx_site_config_preserves_file_when_unchanged(
        self,
        which,
        run_command,
    ):
        stdout = io.StringIO()
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            site_path = tmp_path / "default"
            conf_d_path = tmp_path / "conf.d"
            conf_d_path.mkdir(parents=True, exist_ok=True)
            site_path.write_text(
                prod.build_nginx_site_config(
                    public_url="https://hermestoken.top",
                    app_port="3000",
                ),
                encoding="utf-8",
            )
            changed = prod.sync_nginx_site_config(
                public_url="https://hermestoken.top",
                env_values={"APP_PORT": "3000"},
                output=stdout,
                site_path=site_path,
                conf_d_path=conf_d_path,
            )

        self.assertTrue(changed)
        which.assert_called_once_with("nginx")
        self.assertEqual(run_command.call_count, 2)
        self.assertIn("already up to date", stdout.getvalue())

    @mock.patch("prod.shutil.which", return_value=None)
    def test_sync_nginx_site_config_skips_when_nginx_missing(self, which):
        stdout = io.StringIO()
        changed = prod.sync_nginx_site_config(
            public_url="https://hermestoken.top",
            env_values={"APP_PORT": "3000"},
            output=stdout,
        )

        self.assertFalse(changed)
        which.assert_called_once_with("nginx")
        self.assertIn("skipped nginx site config sync", stdout.getvalue())

    @mock.patch("prod.poll_http_until_healthy")
    @mock.patch("prod.remove_legacy_compose_containers")
    @mock.patch("prod.run_command")
    @mock.patch("prod.resolve_application_version", return_value="e3f7bef8-dirty")
    @mock.patch("prod.prepare_frontend_dist_for_docker_packaging")
    @mock.patch("prod.require_docker_and_compose")
    def test_run_stack_prepares_frontend_and_uses_prebuilt_compose_build(
        self,
        require_docker_and_compose,
        prepare_frontend_dist_for_docker_packaging,
        resolve_application_version,
        run_command,
        remove_legacy_compose_containers,
        poll_http_until_healthy,
    ):
        repo_root = Path(__file__).resolve().parents[2]
        compose_file = repo_root / "docker-compose.prod.yml"
        env_file = repo_root / ".env.production"
        stdout = io.StringIO()

        prod.run_stack(
            action_label="deploy",
            compose_file_path=compose_file,
            env_file_path=env_file,
            local_health_url="http://127.0.0.1:3000/api/status",
            output=stdout,
            repo_root=repo_root,
        )

        require_docker_and_compose.assert_called_once_with()
        prepare_frontend_dist_for_docker_packaging.assert_called_once_with(output=stdout, repo_root=repo_root)
        remove_legacy_compose_containers.assert_called_once_with(
            legacy_project_name="hermestoken",
            compose_file_path=compose_file,
            container_names=prod.PROD_CONTAINER_NAMES,
            output=stdout,
            repo_root=repo_root,
        )
        run_command.assert_called_once_with(
            [
                "docker",
                "compose",
                "--env-file",
                str(env_file),
                "-f",
                str(compose_file),
                "up",
                "-d",
                "--build",
            ],
            check=True,
            stream_output=True,
            cwd=repo_root,
            env={"WEB_DIST_STRATEGY": "prebuilt", "APP_VERSION": "e3f7bef8-dirty"},
            stdout_stream=stdout,
        )
        resolve_application_version.assert_called_once_with(repo_root=repo_root)
        poll_http_until_healthy.assert_called_once_with(
            "http://127.0.0.1:3000/api/status",
            timeout_seconds=prod.DEFAULT_HEALTHCHECK_TIMEOUT_SECONDS,
            interval_seconds=prod.DEFAULT_HEALTHCHECK_INTERVAL_SECONDS,
        )
        self.assertIn("[ok] Production deploy healthy", stdout.getvalue())

    @mock.patch("prod.poll_http_until_healthy")
    @mock.patch("prod.run_command")
    def test_set_public_url_updates_server_address_and_restarts_app(
        self,
        run_command,
        poll_http_until_healthy,
    ):
        repo_root = Path(__file__).resolve().parents[2]
        compose_file = repo_root / "docker-compose.prod.yml"
        env_file = repo_root / ".env.production"
        stdout = io.StringIO()

        prod.set_public_url(
            compose_file_path=compose_file,
            env_file_path=env_file,
            public_url="https://hermestoken.top",
            local_health_url="http://127.0.0.1:3000/api/status",
            output=stdout,
            repo_root=repo_root,
        )

        self.assertEqual(run_command.call_count, 2)
        sql_call = run_command.call_args_list[0]
        self.assertEqual(
            sql_call.args[0][:10],
            [
                "docker",
                "compose",
                "--env-file",
                str(env_file),
                "-f",
                str(compose_file),
                "exec",
                "-T",
                "postgres",
                "psql",
            ],
        )
        sql_command = sql_call.args[0]
        self.assertIn("https://hermestoken.top", sql_command[-1])
        self.assertIn("hermestoken.top", sql_command[-1])

        restart_call = run_command.call_args_list[1]
        self.assertEqual(
            restart_call,
            mock.call(
                [
                    "docker",
                    "compose",
                    "--env-file",
                    str(env_file),
                    "-f",
                    str(compose_file),
                    "restart",
                    "new-api",
                ],
                check=True,
                stream_output=True,
                cwd=repo_root,
                stdout_stream=stdout,
            ),
        )
        poll_http_until_healthy.assert_called_once_with(
            "http://127.0.0.1:3000/api/status",
            timeout_seconds=prod.DEFAULT_HEALTHCHECK_TIMEOUT_SECONDS,
            interval_seconds=prod.DEFAULT_HEALTHCHECK_INTERVAL_SECONDS,
        )
        self.assertIn("Updated ServerAddress to: https://hermestoken.top", stdout.getvalue())

    @mock.patch("prod.load_env_file", return_value={"APP_PORT": "3000"})
    @mock.patch("prod.sync_nginx_site_config")
    @mock.patch("prod.set_public_url")
    @mock.patch("prod.run_stack")
    def test_main_update_dispatches_stack_and_public_url(
        self,
        run_stack,
        set_public_url,
        sync_nginx_site_config,
        load_env_file,
    ):
        stderr = io.StringIO()
        stdout = io.StringIO()
        with mock.patch(
            "sys.argv",
            [
                "prod.py",
                "update",
                "--domain",
                "https://hermestoken.top",
            ],
        ), mock.patch("sys.stderr", stderr), mock.patch("sys.stdout", stdout):
            exit_code = prod.main()

        self.assertEqual(exit_code, 0)
        load_env_file.assert_called_once()
        run_stack.assert_called_once()
        set_public_url.assert_called_once()
        sync_nginx_site_config.assert_called_once_with(
            public_url="https://hermestoken.top",
            env_values={"APP_PORT": "3000"},
            output=stdout,
            frontend_dist_path=prod.REPO_ROOT / "web" / "dist",
        )

    @mock.patch(
        "prod.run_stack",
        side_effect=launcher_common.LauncherError("deploy failed Next step: check docker"),
    )
    @mock.patch("prod.load_env_file", return_value={"APP_PORT": "3000"})
    def test_main_surfaces_actionable_error(self, _load_env_file, _run_stack):
        stderr = io.StringIO()
        with mock.patch("sys.argv", ["prod.py", "deploy"]), mock.patch("sys.stderr", stderr):
            exit_code = prod.main()

        self.assertEqual(exit_code, 1)
        self.assertIn("[error]", stderr.getvalue())
        self.assertIn("Next step", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
