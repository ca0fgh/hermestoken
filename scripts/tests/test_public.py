import io
import subprocess
import sys
import tempfile
import unittest
from collections import deque
from pathlib import Path
from unittest import mock

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import launcher_common
import local
import public


class PublicLauncherTests(unittest.TestCase):
    def _config(
        self,
        cloudflared_config_path: str,
        *,
        cloudflared_tunnel_token=None,
        timeout_seconds: float = 30,
        interval_seconds: float = 1,
    ):
        return launcher_common.LauncherConfig(
            compose_file="docker-compose.yml",
            local_url="http://localhost:3000",
            public_url="https://pay-local.hermestoken.top",
            cloudflared_tunnel_name="hermestoken-local",
            cloudflared_config_path=cloudflared_config_path,
            cloudflared_tunnel_token=cloudflared_tunnel_token,
            healthcheck_timeout_seconds=timeout_seconds,
            healthcheck_interval_seconds=interval_seconds,
        )

    @mock.patch("public._wait_for_public_readiness")
    @mock.patch("public._start_tunnel_process")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    def test_run_public_stack_delegates_to_local_startup_with_build(
        self,
        require_docker_and_compose,
        run_command,
        poll_http_until_healthy,
        require_cloudflared,
        validate_cfargotunnel_cname,
        start_tunnel_process,
        wait_for_public_readiness,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_config = repo_root / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            config = self._config(str(cloudflared_config))

            tunnel_process = mock.Mock()
            tunnel_log_path = repo_root / "cloudflared.log"
            start_tunnel_process.return_value = (tunnel_process, tunnel_log_path)

            stdout = io.StringIO()
            public.run_public_stack(config, output=stdout, repo_root=repo_root)

        require_cloudflared.assert_called_once_with()
        require_docker_and_compose.assert_called_once_with()
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
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
        start_tunnel_process.assert_called_once_with(
            config,
            cloudflared_token=None,
            cloudflared_config_path=cloudflared_config,
            repo_root=repo_root,
        )
        wait_for_public_readiness.assert_called_once_with(config, tunnel_process)
        self.assertIn("[ok] Containers started", stdout.getvalue())

    @mock.patch("public.poll_http_until_healthy")
    @mock.patch("public.subprocess.Popen")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    def test_main_starts_local_then_tunnel_and_waits_for_public_health(
        self,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
        popen_mock,
        poll_http_until_healthy,
    ):
        call_order = []

        with tempfile.TemporaryDirectory() as tmpdir:
            cloudflared_config = Path(tmpdir) / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            load_config.return_value = self._config(str(cloudflared_config))

            process = mock.Mock()
            process.poll.return_value = None
            popen_mock.return_value = process

            def _local_side_effect(*args, **kwargs):
                call_order.append("local")
                return None

            def _popen_side_effect(*args, **kwargs):
                call_order.append("popen")
                return process

            def _poll_side_effect(*args, **kwargs):
                call_order.append("poll")
                return None

            run_local_stack.side_effect = _local_side_effect
            popen_mock.side_effect = _popen_side_effect
            poll_http_until_healthy.side_effect = _poll_side_effect

            stdout = io.StringIO()
            with mock.patch("sys.stdout", stdout):
                exit_code = public.main()

        self.assertEqual(exit_code, 0)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        run_local_stack.assert_called_once_with(
            load_config.return_value,
            output=stdout,
            repo_root=public.REPO_ROOT,
        )
        popen_mock.assert_called_once()
        popen_args, popen_kwargs = popen_mock.call_args
        self.assertEqual(
            popen_args[0],
            [
                "cloudflared",
                "tunnel",
                "--config",
                str(cloudflared_config),
                "run",
                "hermestoken-local",
            ],
        )
        self.assertEqual(popen_kwargs["cwd"], str(public.REPO_ROOT))
        self.assertEqual(popen_kwargs["stderr"], subprocess.STDOUT)
        self.assertEqual(popen_kwargs["text"], True)
        self.assertEqual(popen_kwargs["bufsize"], 1)
        self.assertNotEqual(popen_kwargs["stdout"], subprocess.DEVNULL)
        poll_http_until_healthy.assert_called_once_with(
            "https://pay-local.hermestoken.top",
            timeout_seconds=5.0,
            interval_seconds=0.2,
        )
        self.assertEqual(call_order, ["local", "popen", "poll"])
        self.assertIn("[ok] cloudflared available", stdout.getvalue())
        self.assertIn("[ok] Public service healthy: https://pay-local.hermestoken.top", stdout.getvalue())

    @mock.patch("public.poll_http_until_healthy")
    @mock.patch("public.subprocess.Popen")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    def test_main_supports_cloudflared_token_mode_without_local_config_file(
        self,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
        popen_mock,
        poll_http_until_healthy,
    ):
        load_config.return_value = self._config(
            "/no/such/cloudflared-config.yml",
            cloudflared_tunnel_token="secret-token",
        )

        process = mock.Mock()
        process.poll.return_value = None
        popen_mock.return_value = process
        poll_http_until_healthy.return_value = None

        stdout = io.StringIO()
        with mock.patch("sys.stdout", stdout):
            exit_code = public.main()

        self.assertEqual(exit_code, 0)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_not_called()
        run_local_stack.assert_called_once_with(
            load_config.return_value,
            output=stdout,
            repo_root=public.REPO_ROOT,
        )
        popen_mock.assert_called_once()
        popen_args, _ = popen_mock.call_args
        self.assertEqual(
            popen_args[0],
            [
                "cloudflared",
                "tunnel",
                "run",
                "--token",
                "secret-token",
            ],
        )
        self.assertIn("[ok] cloudflared token auth configured", stdout.getvalue())
        self.assertIn("[ok] Public hostname configured in Cloudflare: pay-local.hermestoken.top", stdout.getvalue())
        self.assertIn("[ok] Public service healthy: https://pay-local.hermestoken.top", stdout.getvalue())

    @mock.patch("public.poll_http_until_healthy")
    @mock.patch("public.subprocess.Popen")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    def test_main_supports_cloudflared_token_file_mode_without_cname_check(
        self,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
        popen_mock,
        poll_http_until_healthy,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            token_path = Path(tmpdir) / "hermestoken-local.token"
            token_path.write_text("secret-token-from-file\n", encoding="utf-8")
            load_config.return_value = self._config(
                "/no/such/cloudflared-config.yml",
                timeout_seconds=30,
                interval_seconds=1,
            )
            load_config.return_value = launcher_common.LauncherConfig(
                compose_file="docker-compose.yml",
                local_url="http://localhost:3000",
                public_url="https://pay-local.hermestoken.top",
                cloudflared_tunnel_name="hermestoken-local",
                cloudflared_config_path="/no/such/cloudflared-config.yml",
                cloudflared_tunnel_token="",
                cloudflared_tunnel_token_path=str(token_path),
                healthcheck_timeout_seconds=30,
                healthcheck_interval_seconds=1,
            )

            process = mock.Mock()
            process.poll.return_value = None
            popen_mock.return_value = process
            poll_http_until_healthy.return_value = None

            stdout = io.StringIO()
            with mock.patch("sys.stdout", stdout):
                exit_code = public.main()

        self.assertEqual(exit_code, 0)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_not_called()
        run_local_stack.assert_called_once()
        popen_args, _ = popen_mock.call_args
        self.assertEqual(
            popen_args[0],
            [
                "cloudflared",
                "tunnel",
                "run",
                "--token",
                "secret-token-from-file",
            ],
        )
        self.assertIn("[ok] Public hostname configured in Cloudflare: pay-local.hermestoken.top", stdout.getvalue())

    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    def test_main_fails_when_cloudflared_config_is_missing(
        self,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
    ):
        missing = "/no/such/cloudflared-config.yml"
        load_config.return_value = self._config(missing)

        stderr = io.StringIO()
        stdout = io.StringIO()
        with mock.patch("sys.stderr", stderr), mock.patch("sys.stdout", stdout):
            exit_code = public.main()

        self.assertEqual(exit_code, 1)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_not_called()
        run_local_stack.assert_not_called()
        self.assertIn("cloudflared", stderr.getvalue())
        self.assertIn("config", stderr.getvalue())
        self.assertIn(missing, stderr.getvalue())

    @mock.patch(
        "public.poll_http_until_healthy",
        side_effect=launcher_common.LauncherError("Health check timed out for https://pay-local.hermestoken.top."),
    )
    @mock.patch("public.subprocess.Popen")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    def test_main_terminates_tunnel_process_if_public_health_fails(
        self,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
        popen_mock,
        _poll_http_until_healthy,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            cloudflared_config = Path(tmpdir) / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            load_config.return_value = self._config(str(cloudflared_config), timeout_seconds=0.01, interval_seconds=0)

            process = mock.Mock()
            process.poll.return_value = None
            process.wait.side_effect = [subprocess.TimeoutExpired(cmd="cloudflared", timeout=5), None]
            popen_mock.return_value = process

            stderr = io.StringIO()
            stdout = io.StringIO()
            with mock.patch("sys.stderr", stderr), mock.patch("sys.stdout", stdout):
                exit_code = public.main()

        self.assertEqual(exit_code, 1)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        run_local_stack.assert_called_once()
        process.terminate.assert_called_once_with()
        process.kill.assert_called_once_with()
        self.assertGreaterEqual(process.wait.call_count, 2)
        self.assertIn("timed out", stderr.getvalue())

    @mock.patch(
        "public.poll_http_until_healthy",
        side_effect=launcher_common.LauncherError("Health check timed out for https://pay-local.hermestoken.top."),
    )
    @mock.patch("public.subprocess.Popen")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    def test_main_returns_actionable_error_when_tunnel_shutdown_double_timeout(
        self,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
        popen_mock,
        _poll_http_until_healthy,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            cloudflared_config = Path(tmpdir) / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            load_config.return_value = self._config(str(cloudflared_config), timeout_seconds=0.01, interval_seconds=0)

            process = mock.Mock()
            process.poll.return_value = None
            process.wait.side_effect = [
                subprocess.TimeoutExpired(cmd="cloudflared", timeout=5),
                subprocess.TimeoutExpired(cmd="cloudflared", timeout=5),
            ]
            popen_mock.return_value = process

            stderr = io.StringIO()
            stdout = io.StringIO()
            with mock.patch("sys.stderr", stderr), mock.patch("sys.stdout", stdout):
                exit_code = public.main()

        self.assertEqual(exit_code, 1)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        run_local_stack.assert_called_once()
        process.terminate.assert_called_once_with()
        process.kill.assert_called_once_with()
        self.assertEqual(process.wait.call_count, 2)
        self.assertIn("[error]", stderr.getvalue())
        self.assertIn("Health check timed out", stderr.getvalue())

    @mock.patch("public.poll_http_until_healthy", side_effect=launcher_common.LauncherError("Health check timed out for https://pay-local.hermestoken.top."))
    @mock.patch("public.subprocess.Popen")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    def test_main_fails_fast_when_tunnel_process_exits_during_public_wait(
        self,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
        popen_mock,
        _poll_http_until_healthy,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            cloudflared_config = Path(tmpdir) / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            load_config.return_value = self._config(str(cloudflared_config), timeout_seconds=0.05, interval_seconds=0)

            process = mock.Mock()
            process.poll.side_effect = [None, 7, 7, 7]
            popen_mock.return_value = process

            stderr = io.StringIO()
            stdout = io.StringIO()
            with mock.patch("sys.stderr", stderr), mock.patch("sys.stdout", stdout):
                exit_code = public.main()

        self.assertEqual(exit_code, 1)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        run_local_stack.assert_called_once()
        process.terminate.assert_not_called()
        process.kill.assert_not_called()
        self.assertIn("Tunnel process exited unexpectedly", stderr.getvalue())
        self.assertIn("exit code 7", stderr.getvalue())

    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("public.load_launcher_config")
    @mock.patch("public.subprocess.Popen", side_effect=OSError("spawn failed"))
    def test_main_returns_actionable_error_when_tunnel_process_fails_to_start(
        self,
        _popen_mock,
        load_config,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            cloudflared_config = Path(tmpdir) / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            load_config.return_value = self._config(str(cloudflared_config))

            stderr = io.StringIO()
            stdout = io.StringIO()
            with mock.patch("sys.stderr", stderr), mock.patch("sys.stdout", stdout):
                exit_code = public.main()

        self.assertEqual(exit_code, 1)
        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        run_local_stack.assert_called_once()
        self.assertIn("Unable to start Cloudflare tunnel process", stderr.getvalue())
        self.assertIn("spawn failed", stderr.getvalue())

    @mock.patch(
        "public.poll_http_until_healthy",
        side_effect=[
            launcher_common.LauncherError("probe1"),
            launcher_common.LauncherError("probe2"),
        ],
    )
    def test_wait_for_public_readiness_uses_bounded_probe_windows_and_reports_early_exit(self, poll_http_until_healthy):
        config = self._config("/tmp/config.yml", timeout_seconds=5, interval_seconds=30)
        process = mock.Mock()
        process.poll.side_effect = [None, None, 23]
        log_buffer = deque(["cloudflared booting", "error: origin refused"], maxlen=20)

        with self.assertRaises(launcher_common.LauncherError) as ctx:
            with mock.patch("public.time.sleep"):
                public._wait_for_public_readiness(config, process, log_buffer=log_buffer)

        self.assertIn("Tunnel process exited unexpectedly", str(ctx.exception))
        self.assertIn("exit code 23", str(ctx.exception))
        self.assertIn("origin refused", str(ctx.exception))
        self.assertEqual(poll_http_until_healthy.call_count, 2)
        for call in poll_http_until_healthy.call_args_list:
            self.assertLessEqual(call.kwargs["timeout_seconds"], 5.0)
            self.assertLessEqual(call.kwargs["interval_seconds"], 0.2)


if __name__ == "__main__":
    unittest.main()
