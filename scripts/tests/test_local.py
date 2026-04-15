import io
import os
import subprocess
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
        self.assertIn("WEB_DIST_STRATEGY: ${WEB_DIST_STRATEGY:-prebuilt}", compose_text)

    @mock.patch("local.run_browser_smoke_check")
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.remove_legacy_compose_containers")
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    @mock.patch("local.load_launcher_config")
    def test_main_runs_local_stack_and_waits_for_health(
        self,
        load_config,
        require_docker_and_compose,
        run_command,
        remove_legacy_compose_containers,
        poll_http_until_healthy,
        run_browser_smoke_check,
    ):
        repo_root = Path(__file__).resolve().parents[2]
        config = self._config()
        load_config.return_value = config

        stdout = io.StringIO()
        with mock.patch("sys.stdout", stdout):
            exit_code = local.main()

        self.assertEqual(exit_code, 0)
        require_docker_and_compose.assert_called_once_with()
        remove_legacy_compose_containers.assert_called_once_with(
            legacy_project_name="hermestoken",
            compose_file_path=repo_root / "docker-compose.yml",
            container_names=local.LOCAL_CONTAINER_NAMES,
            output=stdout,
            repo_root=repo_root,
        )
        run_command.assert_called_once_with(
            ["docker", "compose", "-f", str(repo_root / "docker-compose.yml"), "up", "-d", "--build"],
            check=True,
            stream_output=True,
            cwd=repo_root,
            stdout_stream=stdout,
        )
        poll_http_until_healthy.assert_called_once_with(
            "http://localhost:3000",
            timeout_seconds=30,
            interval_seconds=1,
        )
        run_browser_smoke_check.assert_called_once_with("http://localhost:3000", output=stdout)
        self.assertIn("http://localhost:3000", stdout.getvalue())
        self.assertIn("[ok] Docker available", stdout.getvalue())
        self.assertIn("[ok] Containers started", stdout.getvalue())

    @mock.patch("local.require_docker_and_compose", side_effect=launcher_common.LauncherError("docker missing Next step: install docker"))
    @mock.patch("local.load_launcher_config")
    def test_main_returns_non_zero_with_actionable_error(self, load_config, _require):
        load_config.return_value = self._config()

        stderr = io.StringIO()
        with mock.patch("sys.stderr", stderr):
            exit_code = local.main()

        self.assertEqual(exit_code, 1)
        self.assertIn("[error]", stderr.getvalue())
        self.assertIn("Next step", stderr.getvalue())

    @mock.patch(
        "local.poll_http_until_healthy",
        side_effect=launcher_common.LauncherError("Health check timed out for http://localhost:3000."),
    )
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    @mock.patch("local.load_launcher_config")
    def test_main_prints_container_status_when_local_health_check_fails(
        self,
        load_config,
        require_docker_and_compose,
        run_command,
        _poll_http_until_healthy,
    ):
        repo_root = Path(__file__).resolve().parents[2]
        load_config.return_value = self._config()
        run_command.side_effect = [
            mock.Mock(returncode=0, stdout="started\n"),
            mock.Mock(returncode=0, stdout="NAME   STATE\nweb    running\n"),
        ]

        stdout = io.StringIO()
        stderr = io.StringIO()
        with mock.patch("sys.stdout", stdout), mock.patch("sys.stderr", stderr):
            exit_code = local.main()

        self.assertEqual(exit_code, 1)
        require_docker_and_compose.assert_called_once_with()
        self.assertEqual(run_command.call_count, 2)
        run_command.assert_has_calls(
            [
                mock.call(
                    ["docker", "compose", "-f", str(repo_root / "docker-compose.yml"), "up", "-d", "--build"],
                    check=True,
                    stream_output=True,
                    cwd=repo_root,
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["docker", "compose", "-f", str(repo_root / "docker-compose.yml"), "ps"],
                    check=False,
                    stream_output=False,
                    cwd=repo_root,
                ),
            ]
        )
        self.assertIn("[info] Recent container status", stdout.getvalue())
        self.assertIn("web    running", stdout.getvalue())
        self.assertIn("[error]", stderr.getvalue())

    @mock.patch(
        "local.poll_http_until_healthy",
        side_effect=launcher_common.LauncherError("Health check timed out for http://localhost:3000."),
    )
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    @mock.patch("local.load_launcher_config")
    def test_main_surfaces_ps_stderr_when_ps_command_fails(
        self,
        load_config,
        _require_docker_and_compose,
        run_command,
        _poll_http_until_healthy,
    ):
        load_config.return_value = self._config()
        run_command.side_effect = [
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="started\n", stderr=""),
            subprocess.CompletedProcess(args=["docker"], returncode=1, stdout="", stderr="cannot connect daemon"),
        ]

        stdout = io.StringIO()
        stderr = io.StringIO()
        with mock.patch("sys.stdout", stdout), mock.patch("sys.stderr", stderr):
            exit_code = local.main()

        self.assertEqual(exit_code, 1)
        self.assertIn("cannot connect daemon", stdout.getvalue())
        self.assertNotIn("No container status output was returned", stdout.getvalue())
        self.assertIn("[error]", stderr.getvalue())

    @mock.patch("local.poll_http_until_healthy", side_effect=launcher_common.LauncherError("Health check timed out for http://localhost:3000."))
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    def test_run_local_stack_handles_empty_ps_output(self, _require_docker_and_compose, run_command, _poll_http_until_healthy):
        run_command.side_effect = [
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="started\n", stderr=""),
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="", stderr=""),
        ]
        stdout = io.StringIO()

        with self.assertRaises(launcher_common.LauncherError):
            local.run_local_stack(self._config(), output=stdout)

        self.assertIn("Recent container status", stdout.getvalue())
        self.assertIn("No container status output was returned", stdout.getvalue())

    @mock.patch("local.run_browser_smoke_check")
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.remove_legacy_compose_containers")
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    def test_run_local_stack_uses_absolute_compose_path_without_rebasing(
        self,
        _require_docker_and_compose,
        run_command,
        remove_legacy_compose_containers,
        poll_http_until_healthy,
        run_browser_smoke_check,
    ):
        absolute_compose = Path("/tmp/hermestoken-compose.yml")
        custom_repo_root = Path("/tmp/custom-repo-root")
        stdout = io.StringIO()

        local.run_local_stack(self._config(compose_file=str(absolute_compose)), output=stdout, repo_root=custom_repo_root)

        remove_legacy_compose_containers.assert_called_once_with(
            legacy_project_name="hermestoken",
            compose_file_path=absolute_compose,
            container_names=local.LOCAL_CONTAINER_NAMES,
            output=stdout,
            repo_root=custom_repo_root,
        )
        run_command.assert_called_once_with(
            ["docker", "compose", "-f", str(absolute_compose), "up", "-d", "--build"],
            check=True,
            stream_output=True,
            cwd=custom_repo_root,
            stdout_stream=stdout,
        )
        poll_http_until_healthy.assert_called_once()
        run_browser_smoke_check.assert_called_once_with("http://localhost:3000", output=stdout)

    @mock.patch("local.run_browser_smoke_check", create=True)
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    def test_run_local_stack_runs_browser_smoke_check_after_http_health(
        self,
        _require_docker_and_compose,
        run_command,
        poll_http_until_healthy,
        run_browser_smoke_check,
    ):
        stdout = io.StringIO()
        call_order = []

        def mark_health(*args, **kwargs):
            call_order.append("health")
            return None

        def mark_browser(*args, **kwargs):
            call_order.append("browser")
            return None

        poll_http_until_healthy.side_effect = mark_health
        run_browser_smoke_check.side_effect = mark_browser

        local.run_local_stack(self._config(), output=stdout)

        poll_http_until_healthy.assert_called_once_with(
            "http://localhost:3000",
            timeout_seconds=30,
            interval_seconds=1,
        )
        run_browser_smoke_check.assert_called_once_with(
            "http://localhost:3000",
            output=stdout,
        )
        self.assertEqual(call_order, ["health", "browser"])

    @mock.patch("local.run_browser_smoke_check", create=True)
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    def test_run_local_stack_treats_browser_skip_as_non_fatal(
        self,
        _require_docker_and_compose,
        _run_command,
        _poll_http_until_healthy,
        run_browser_smoke_check,
    ):
        stdout = io.StringIO()

        def write_skip(url, *, output, timeout_seconds=15.0):
            output.write(f"[warn] Browser smoke check skipped for {url}: Chrome/Chromium not found\n")
            return None

        run_browser_smoke_check.side_effect = write_skip

        local.run_local_stack(self._config(), output=stdout)

        self.assertIn("[warn] Browser smoke check skipped for http://localhost:3000", stdout.getvalue())
        self.assertIn("[ok] Local service healthy: http://localhost:3000", stdout.getvalue())

    @mock.patch(
        "local.run_browser_smoke_check",
        side_effect=launcher_common.LauncherError("Browser smoke check failed for http://localhost:3000: root element remained empty"),
    )
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    def test_run_local_stack_prints_container_status_when_browser_smoke_check_fails(
        self,
        _require_docker_and_compose,
        run_command,
        _poll_http_until_healthy,
        _run_browser_smoke_check,
    ):
        run_command.side_effect = [
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="started\n", stderr=""),
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="NAME   STATE\nweb    running\n", stderr=""),
        ]
        stdout = io.StringIO()

        with self.assertRaises(launcher_common.LauncherError):
            local.run_local_stack(self._config(), output=stdout)

        self.assertIn("[info] Recent container status", stdout.getvalue())
        self.assertIn("web    running", stdout.getvalue())


class LauncherBrowserDiscoveryTests(unittest.TestCase):
    @mock.patch("launcher_common.shutil.which")
    @mock.patch("launcher_common.os.access")
    @mock.patch("launcher_common.os.path.isfile")
    def test_find_browser_executable_prefers_absolute_executable(self, isfile_mock, access_mock, which_mock):
        absolute_browser = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

        isfile_mock.side_effect = lambda path: path == absolute_browser
        access_mock.side_effect = lambda path, mode: path == absolute_browser and mode == os.X_OK
        which_mock.return_value = "/usr/bin/chromium"

        resolved = launcher_common.find_browser_executable((absolute_browser, "chromium"))

        self.assertEqual(resolved, absolute_browser)
        which_mock.assert_not_called()

    @mock.patch("launcher_common.shutil.which", return_value="/usr/bin/chromium")
    @mock.patch("launcher_common.os.access", return_value=False)
    @mock.patch("launcher_common.os.path.isfile", return_value=False)
    def test_find_browser_executable_falls_back_to_path_lookup(self, _isfile_mock, _access_mock, which_mock):
        resolved = launcher_common.find_browser_executable(("chromium",))

        self.assertEqual(resolved, "/usr/bin/chromium")
        which_mock.assert_called_once_with("chromium")

    @mock.patch("launcher_common.subprocess.Popen")
    @mock.patch("launcher_common.find_browser_executable", return_value=None)
    def test_run_browser_smoke_check_warns_and_returns_when_browser_missing(self, _find_browser_executable, popen_mock):
        stdout = io.StringIO()

        launcher_common.run_browser_smoke_check("http://localhost:3000", output=stdout)

        self.assertIn(
            "[warn] Browser smoke check skipped for http://localhost:3000: Chrome/Chromium not found",
            stdout.getvalue(),
        )
        popen_mock.assert_not_called()


if __name__ == "__main__":
    unittest.main()
