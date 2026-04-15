import io
import sys
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

    @mock.patch("prod.poll_http_until_healthy")
    @mock.patch("prod.remove_legacy_compose_containers")
    @mock.patch("prod.run_command")
    @mock.patch("prod.prepare_frontend_dist_for_docker_packaging")
    @mock.patch("prod.require_docker_and_compose")
    def test_run_stack_prepares_frontend_and_uses_prebuilt_compose_build(
        self,
        require_docker_and_compose,
        prepare_frontend_dist_for_docker_packaging,
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
            env={"WEB_DIST_STRATEGY": "prebuilt"},
            stdout_stream=stdout,
        )
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
    @mock.patch("prod.set_public_url")
    @mock.patch("prod.run_stack")
    def test_main_update_dispatches_stack_and_public_url(
        self,
        run_stack,
        set_public_url,
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
