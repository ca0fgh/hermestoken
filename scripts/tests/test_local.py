import io
import os
import sys
import unittest
from pathlib import Path
from unittest import mock

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import launcher_common
import local


class LocalLauncherTests(unittest.TestCase):
    def _config(self, compose_file="docker-compose.yml"):
        return launcher_common.LauncherConfig(
            compose_file=compose_file,
            local_url="http://localhost:3000",
            public_url="https://pay-local.hermestoken.top",
            cloudflared_tunnel_name="hermestoken-local",
            cloudflared_config_path="~/.cloudflared/config.yml",
            healthcheck_timeout_seconds=30,
            healthcheck_interval_seconds=1,
        )

    def test_local_compose_uses_dedicated_project_name_and_prebuilt_frontend_by_default(self):
        repo_root = Path(__file__).resolve().parents[2]
        compose_text = (repo_root / "docker-compose.yml").read_text(encoding="utf-8")

        self.assertRegex(compose_text, r"(?m)^name:\s+hermestoken-local$")
        self.assertIn("name: hermestoken_pg_data", compose_text)
        self.assertIn("external: true", compose_text)
        self.assertIn("WEB_DIST_STRATEGY: ${WEB_DIST_STRATEGY:-prebuilt}", compose_text)
        self.assertIn("APP_VERSION: ${APP_VERSION:-}", compose_text)

    @mock.patch("local.run_browser_smoke_check")
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.ensure_named_docker_volume")
    @mock.patch("local.remove_legacy_compose_containers")
    @mock.patch("local.run_command")
    @mock.patch("local.resolve_application_version", return_value="e3f7bef8-dirty")
    @mock.patch("local.prepare_frontend_dist_for_docker_packaging")
    @mock.patch("local.require_docker_and_compose")
    def test_run_local_stack_builds_frontend_on_host_and_packages_prebuilt_dist(
        self,
        require_docker_and_compose,
        prepare_frontend_dist_for_docker_packaging,
        resolve_application_version,
        run_command,
        remove_legacy_compose_containers,
        ensure_named_docker_volume,
        poll_http_until_healthy,
        run_browser_smoke_check,
    ):
        repo_root = Path(__file__).resolve().parents[2]
        stdout = io.StringIO()

        local.run_local_stack(self._config(), output=stdout, repo_root=repo_root, action_label="update")

        require_docker_and_compose.assert_called_once_with()
        prepare_frontend_dist_for_docker_packaging.assert_called_once_with(output=stdout, repo_root=repo_root)
        remove_legacy_compose_containers.assert_called_once_with(
            legacy_project_name="hermestoken",
            compose_file_path=repo_root / "docker-compose.yml",
            container_names=local.LOCAL_CONTAINER_NAMES,
            output=stdout,
            repo_root=repo_root,
        )
        ensure_named_docker_volume.assert_called_once_with(
            "hermestoken_pg_data",
            output=stdout,
            repo_root=repo_root,
        )
        run_command.assert_called_once_with(
            [
                "docker",
                "compose",
                "-f",
                str(repo_root / "docker-compose.yml"),
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
            "http://localhost:3000",
            timeout_seconds=30,
            interval_seconds=1,
        )
        run_browser_smoke_check.assert_called_once_with("http://localhost:3000", output=stdout)
        self.assertIn("[ok] Local update healthy", stdout.getvalue())

    @mock.patch.dict(os.environ, {"WEB_DIST_STRATEGY": "build"}, clear=True)
    def test_run_local_stack_rejects_docker_side_frontend_build_modes(self):
        with self.assertRaises(launcher_common.LauncherError) as context:
            local.resolve_web_dist_strategy()

        self.assertIn("avoid Docker-side Vite OOM", str(context.exception))

    @mock.patch("local.run_local_stack")
    @mock.patch("local.load_launcher_config")
    def test_main_defaults_to_deploy_when_no_command_is_provided(self, load_config, run_local_stack):
        load_config.return_value = self._config()

        with mock.patch("sys.argv", ["local.py"]):
            exit_code = local.main()

        self.assertEqual(exit_code, 0)
        run_local_stack.assert_called_once_with(load_config.return_value, output=sys.stdout, action_label="deploy")

    @mock.patch("local.run_local_stack")
    @mock.patch("local.load_launcher_config")
    def test_main_accepts_update_command(self, load_config, run_local_stack):
        load_config.return_value = self._config()

        with mock.patch("sys.argv", ["local.py", "update"]):
            exit_code = local.main()

        self.assertEqual(exit_code, 0)
        run_local_stack.assert_called_once_with(load_config.return_value, output=sys.stdout, action_label="update")


if __name__ == "__main__":
    unittest.main()
