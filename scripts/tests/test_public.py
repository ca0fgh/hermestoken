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

    @mock.patch("public.run_browser_smoke_check")
    @mock.patch("public._wait_for_public_readiness")
    @mock.patch("public._start_tunnel_process")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    @mock.patch("local.run_browser_smoke_check")
    @mock.patch("local.poll_http_until_healthy")
    @mock.patch("local.run_command")
    @mock.patch("local.require_docker_and_compose")
    def test_run_public_stack_delegates_to_local_startup_with_build(
        self,
        require_docker_and_compose,
        run_command,
        poll_http_until_healthy,
        run_local_browser_smoke_check,
        require_cloudflared,
        validate_cfargotunnel_cname,
        start_tunnel_process,
        wait_for_public_readiness,
        run_public_browser_smoke_check,
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
        run_local_browser_smoke_check.assert_called_once_with("http://localhost:3000", output=stdout)
        run_public_browser_smoke_check.assert_called_once_with("https://pay-local.hermestoken.top", output=stdout)
        start_tunnel_process.assert_called_once_with(
            config,
            cloudflared_token=None,
            cloudflared_config_path=cloudflared_config,
            repo_root=repo_root,
        )
        wait_for_public_readiness.assert_called_once_with(config, tunnel_process)
        self.assertIn("[ok] Containers started", stdout.getvalue())

    @mock.patch("public.run_browser_smoke_check", create=True)
    @mock.patch("public._wait_for_public_readiness")
    @mock.patch("public._start_tunnel_process")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    def test_run_public_stack_runs_browser_smoke_check_after_public_http_health(
        self,
        require_cloudflared,
        validate_cfargotunnel_cname,
        run_local_stack,
        start_tunnel_process,
        wait_for_public_readiness,
        run_browser_smoke_check,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_config = repo_root / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            config = self._config(str(cloudflared_config))
            tunnel_process = mock.Mock()
            start_tunnel_process.return_value = (tunnel_process, repo_root / "cloudflared.log")
            stdout = io.StringIO()
            call_order = []

            def mark_health(*args, **kwargs):
                call_order.append("health")
                return None

            def mark_browser(*args, **kwargs):
                call_order.append("browser")
                return None

            wait_for_public_readiness.side_effect = mark_health
            run_browser_smoke_check.side_effect = mark_browser

            public.run_public_stack(config, output=stdout, repo_root=repo_root)

        require_cloudflared.assert_called_once_with()
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        run_local_stack.assert_called_once_with(config, output=stdout, repo_root=repo_root)
        wait_for_public_readiness.assert_called_once_with(config, tunnel_process)
        run_browser_smoke_check.assert_called_once_with(
            "https://pay-local.hermestoken.top",
            output=stdout,
        )
        self.assertEqual(call_order, ["health", "browser"])

    @mock.patch(
        "public.run_browser_smoke_check",
        side_effect=launcher_common.LauncherError(
            "Browser smoke check failed for https://pay-local.hermestoken.top: root element remained empty"
        ),
    )
    @mock.patch("public._wait_for_public_readiness")
    @mock.patch("public._start_tunnel_process")
    @mock.patch("public.local.run_local_stack")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.require_cloudflared")
    def test_run_public_stack_stops_tunnel_and_adds_log_context_when_browser_smoke_check_fails(
        self,
        _require_cloudflared,
        _validate_cfargotunnel_cname,
        _run_local_stack,
        start_tunnel_process,
        _wait_for_public_readiness,
        _run_browser_smoke_check,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_config = repo_root / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            tunnel_log_path = repo_root / "cloudflared.log"
            tunnel_log_path.write_text("origin connected\nbrowser failure context\n", encoding="utf-8")
            process = mock.Mock()
            process.poll.return_value = None
            start_tunnel_process.return_value = (process, tunnel_log_path)

            with self.assertRaises(launcher_common.LauncherError) as context:
                public.run_public_stack(self._config(str(cloudflared_config)), output=io.StringIO(), repo_root=repo_root)

        process.terminate.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=5)
        self.assertIn("Browser smoke check failed", str(context.exception))
        self.assertIn("browser failure context", str(context.exception))

    @mock.patch("public.run_browser_smoke_check")
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
        run_browser_smoke_check,
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

            def _browser_side_effect(*args, **kwargs):
                call_order.append("browser")
                return None

            run_local_stack.side_effect = _local_side_effect
            popen_mock.side_effect = _popen_side_effect
            poll_http_until_healthy.side_effect = _poll_side_effect
            run_browser_smoke_check.side_effect = _browser_side_effect

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
        run_browser_smoke_check.assert_called_once_with("https://pay-local.hermestoken.top", output=stdout)
        self.assertEqual(call_order, ["local", "popen", "poll", "browser"])
        self.assertIn("[ok] cloudflared available", stdout.getvalue())
        self.assertIn("[ok] Public service healthy: https://pay-local.hermestoken.top", stdout.getvalue())

    @mock.patch("public.run_browser_smoke_check")
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
        run_browser_smoke_check,
    ):
        load_config.return_value = self._config(
            "/no/such/cloudflared-config.yml",
            cloudflared_tunnel_token="secret-token",
        )

        process = mock.Mock()
        process.poll.return_value = None
        popen_mock.return_value = process
        poll_http_until_healthy.return_value = None
        run_browser_smoke_check.return_value = None

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

    @mock.patch("public.run_browser_smoke_check")
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
        run_browser_smoke_check,
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
            run_browser_smoke_check.return_value = None

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


class LauncherBrowserSmokeCheckTests(unittest.TestCase):
    @mock.patch("launcher_common._recv_websocket_frame", return_value='{"method":"Page.loadEventFired"}')
    @mock.patch("launcher_common.json.loads", return_value={"method": "Page.loadEventFired"})
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.01])
    def test_cdp_wait_for_message_without_filters_returns_next_message(
        self,
        _monotonic_mock,
        json_loads_mock,
        recv_websocket_frame_mock,
    ):
        fake_socket = mock.Mock()

        message = launcher_common._cdp_wait_for_message(fake_socket, timeout_seconds=0.1)

        self.assertEqual(message, {"method": "Page.loadEventFired"})
        recv_websocket_frame_mock.assert_called_once_with(fake_socket)
        json_loads_mock.assert_called_once_with('{"method":"Page.loadEventFired"}')
        fake_socket.settimeout.assert_called_once_with(0.09000000000000001)

    def test_recv_websocket_frame_masks_client_pong_replies(self):
        ping_payload = b"hi"
        frame = bytes([0x89, len(ping_payload)]) + ping_payload

        fake_socket = mock.Mock()
        fake_socket.recv.side_effect = [frame[:2], frame[2:], b"\x81\x02", b"ok"]

        message = launcher_common._recv_websocket_frame(fake_socket)

        self.assertEqual(message, "ok")
        fake_socket.sendall.assert_called_once()
        sent_frame = fake_socket.sendall.call_args.args[0]
        self.assertEqual(sent_frame[0], 0x8A)
        self.assertTrue(sent_frame[1] & 0x80)
        mask = sent_frame[2:6]
        masked_payload = sent_frame[6:]
        self.assertEqual(
            masked_payload,
            bytes(byte ^ mask[index % 4] for index, byte in enumerate(ping_payload)),
        )

    @mock.patch(
        "launcher_common._run_browser_smoke_check_via_cdp",
        return_value=launcher_common.BrowserSmokeCheckResult(status="passed"),
    )
    @mock.patch("launcher_common.shutil.rmtree")
    @mock.patch("launcher_common.tempfile.mkdtemp", return_value="/tmp/launcher-browser-smoke-123")
    @mock.patch("launcher_common.find_browser_executable", return_value="/usr/bin/chromium")
    @mock.patch("launcher_common.subprocess.Popen")
    def test_run_browser_smoke_check_launches_browser_and_cleans_up_on_success(
        self,
        popen_mock,
        _find_browser_executable,
        _mkdtemp_mock,
        rmtree_mock,
        run_via_cdp_mock,
    ):
        process = mock.Mock()
        process.poll.side_effect = [None, 0]
        popen_mock.return_value = process
        stdout = io.StringIO()

        launcher_common.run_browser_smoke_check("https://pay-local.hermestoken.top", output=stdout, timeout_seconds=9.5)

        popen_mock.assert_called_once()
        command = popen_mock.call_args.args[0]
        self.assertEqual(command[0], "/usr/bin/chromium")
        self.assertIn("--headless=new", command)
        self.assertIn("--disable-gpu", command)
        self.assertIn("--remote-debugging-port=0", command)
        self.assertIn("--user-data-dir=/tmp/launcher-browser-smoke-123", command)
        self.assertEqual(command[-1], "about:blank")
        run_via_cdp_mock.assert_called_once_with(
            "https://pay-local.hermestoken.top",
            chrome_process=process,
            profile_dir=Path("/tmp/launcher-browser-smoke-123"),
            timeout_seconds=9.5,
        )
        process.terminate.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=5)
        rmtree_mock.assert_called_once_with(Path("/tmp/launcher-browser-smoke-123"), ignore_errors=True)
        self.assertIn("[ok] Browser smoke check passed: https://pay-local.hermestoken.top", stdout.getvalue())

    @mock.patch(
        "launcher_common._run_browser_smoke_check_via_cdp",
        return_value=launcher_common.BrowserSmokeCheckResult(
            status="failed",
            detail="TypeError: Cannot read properties of undefined",
        ),
    )
    @mock.patch("launcher_common.shutil.rmtree")
    @mock.patch("launcher_common.tempfile.mkdtemp", return_value="/tmp/launcher-browser-smoke-456")
    @mock.patch("launcher_common.find_browser_executable", return_value="/usr/bin/chromium")
    @mock.patch("launcher_common.subprocess.Popen")
    def test_run_browser_smoke_check_raises_launcher_error_with_first_diagnostic(
        self,
        popen_mock,
        _find_browser_executable,
        _mkdtemp_mock,
        rmtree_mock,
        _run_via_cdp_mock,
    ):
        process = mock.Mock()
        process.poll.return_value = 0
        popen_mock.return_value = process

        with self.assertRaises(launcher_common.LauncherError) as context:
            launcher_common.run_browser_smoke_check("http://localhost:3000")

        self.assertIn(
            "Browser smoke check failed for http://localhost:3000: TypeError: Cannot read properties of undefined",
            str(context.exception),
        )
        self.assertIn("Open the URL in a clean browser profile", str(context.exception))
        process.terminate.assert_not_called()
        rmtree_mock.assert_called_once_with(Path("/tmp/launcher-browser-smoke-456"), ignore_errors=True)

    @mock.patch(
        "launcher_common._run_browser_smoke_check_via_cdp",
        return_value=launcher_common.BrowserSmokeCheckResult(
            status="browser_error",
            detail="browser exited before the debugging endpoint became ready",
        ),
    )
    @mock.patch("launcher_common.shutil.rmtree")
    @mock.patch("launcher_common.tempfile.mkdtemp", return_value="/tmp/launcher-browser-smoke-457")
    @mock.patch("launcher_common.find_browser_executable", return_value="/usr/bin/chromium")
    @mock.patch("launcher_common.subprocess.Popen")
    def test_run_browser_smoke_check_raises_browser_environment_error_for_startup_failure(
        self,
        popen_mock,
        _find_browser_executable,
        _mkdtemp_mock,
        rmtree_mock,
        _run_via_cdp_mock,
    ):
        process = mock.Mock()
        process.poll.return_value = 17
        popen_mock.return_value = process

        with self.assertRaises(launcher_common.LauncherError) as context:
            launcher_common.run_browser_smoke_check("http://localhost:3000")

        self.assertIn(
            "Browser smoke check could not start a usable Chrome/Chromium session for http://localhost:3000: browser exited before the debugging endpoint became ready",
            str(context.exception),
        )
        self.assertIn("Verify Chrome/Chromium is installed and launchable", str(context.exception))
        rmtree_mock.assert_called_once_with(Path("/tmp/launcher-browser-smoke-457"), ignore_errors=True)

    @mock.patch(
        "launcher_common._run_browser_smoke_check_via_cdp",
        side_effect=RuntimeError("cdp connection lost"),
    )
    @mock.patch("launcher_common.shutil.rmtree")
    @mock.patch("launcher_common.tempfile.mkdtemp", return_value="/tmp/launcher-browser-smoke-789")
    @mock.patch("launcher_common.find_browser_executable", return_value="/usr/bin/chromium")
    @mock.patch("launcher_common.subprocess.Popen")
    def test_run_browser_smoke_check_kills_browser_when_terminate_wait_times_out(
        self,
        popen_mock,
        _find_browser_executable,
        _mkdtemp_mock,
        rmtree_mock,
        _run_via_cdp_mock,
    ):
        process = mock.Mock()
        process.poll.return_value = None
        process.wait.side_effect = subprocess.TimeoutExpired(cmd="chromium", timeout=5)
        popen_mock.return_value = process

        with self.assertRaises(RuntimeError):
            launcher_common.run_browser_smoke_check("http://localhost:3000")

        process.terminate.assert_called_once_with()
        process.kill.assert_called_once_with()
        rmtree_mock.assert_called_once_with(Path("/tmp/launcher-browser-smoke-789"), ignore_errors=True)

    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.01, 0.02, 0.03, 0.04, 0.10, 0.11])
    @mock.patch(
        "launcher_common._evaluate_browser_expression",
        side_effect=[(4, ""), (5, "<div>ready</div>"), (6, "ready")],
    )
    @mock.patch("launcher_common._cdp_wait_for_message")
    @mock.patch("launcher_common._cdp_request")
    @mock.patch("launcher_common._connect_to_devtools", return_value=mock.Mock())
    @mock.patch(
        "launcher_common._wait_for_page_target",
        return_value=launcher_common._DevToolsTarget(port=41239, websocket_url="ws://127.0.0.1:41239/devtools/page/99"),
    )
    @mock.patch(
        "launcher_common._wait_for_devtools_target",
        return_value=launcher_common._DevToolsTarget(port=41239, websocket_url="ws://127.0.0.1:41239/devtools/browser/1"),
    )
    def test_run_browser_smoke_check_via_cdp_uses_settle_window_and_allows_empty_title(
        self,
        _wait_for_target,
        wait_for_page_target,
        _connect_to_devtools,
        _cdp_request,
        cdp_wait_for_message,
        evaluate_browser_expression,
        _monotonic_mock,
        _sleep_mock,
    ):
        cdp_wait_for_message.side_effect = [
            {"method": "Page.loadEventFired"},
            {
                "method": "Runtime.exceptionThrown",
                "params": {"exceptionDetails": {"text": "TypeError: late startup crash"}},
            },
            launcher_common.LauncherError("timed out while waiting for browser diagnostics"),
        ]

        result = launcher_common._run_browser_smoke_check_via_cdp(
            "http://localhost:3000",
            chrome_process=mock.Mock(),
            profile_dir=Path("/tmp/profile"),
            timeout_seconds=0.1,
        )

        self.assertEqual(result.status, "failed")
        self.assertEqual(result.detail, "TypeError: late startup crash")
        wait_for_page_target.assert_called_once()
        evaluate_browser_expression.assert_called()

    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.01, 0.02, 0.03, 0.09, 0.10])
    @mock.patch(
        "launcher_common._evaluate_browser_expression",
        side_effect=[(4, ""), (5, "<div>ready</div>"), (6, "ready")],
    )
    @mock.patch("launcher_common._cdp_wait_for_message")
    @mock.patch("launcher_common._cdp_request")
    @mock.patch("launcher_common._connect_to_devtools", return_value=mock.Mock())
    @mock.patch(
        "launcher_common._wait_for_page_target",
        return_value=launcher_common._DevToolsTarget(port=41239, websocket_url="ws://127.0.0.1:41239/devtools/page/99"),
    )
    @mock.patch(
        "launcher_common._wait_for_devtools_target",
        return_value=launcher_common._DevToolsTarget(port=41239, websocket_url="ws://127.0.0.1:41239/devtools/browser/1"),
    )
    def test_run_browser_smoke_check_via_cdp_does_not_require_non_empty_title(
        self,
        _wait_for_target,
        wait_for_page_target,
        _connect_to_devtools,
        _cdp_request,
        cdp_wait_for_message,
        evaluate_browser_expression,
        _monotonic_mock,
        _sleep_mock,
    ):
        cdp_wait_for_message.side_effect = [
            {"method": "Page.loadEventFired"},
            launcher_common.LauncherError("timed out while waiting for browser diagnostics"),
        ]

        result = launcher_common._run_browser_smoke_check_via_cdp(
            "http://localhost:3000",
            chrome_process=mock.Mock(),
            profile_dir=Path("/tmp/profile"),
            timeout_seconds=0.1,
        )

        self.assertEqual(result.status, "passed")
        self.assertEqual(result.detail, "")
        wait_for_page_target.assert_called_once()
        evaluate_browser_expression.assert_called()

    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.01, 0.02, 0.03, 0.09, 0.10])
    @mock.patch(
        "launcher_common._evaluate_browser_expression",
        side_effect=[(4, ""), (5, "<div>ready</div>"), (6, "ready")],
    )
    @mock.patch("launcher_common._cdp_wait_for_message")
    @mock.patch("launcher_common._cdp_request")
    @mock.patch("launcher_common._connect_to_devtools", return_value=mock.Mock())
    @mock.patch(
        "launcher_common._wait_for_page_target",
        return_value=launcher_common._DevToolsTarget(port=41239, websocket_url="ws://127.0.0.1:41239/devtools/page/99"),
    )
    @mock.patch(
        "launcher_common._wait_for_devtools_target",
        return_value=launcher_common._DevToolsTarget(port=41239, websocket_url="ws://127.0.0.1:41239/devtools/browser/1"),
    )
    def test_run_browser_smoke_check_via_cdp_treats_settle_timeouterror_as_normal_idle_completion(
        self,
        _wait_for_target,
        wait_for_page_target,
        _connect_to_devtools,
        _cdp_request,
        cdp_wait_for_message,
        evaluate_browser_expression,
        _monotonic_mock,
        _sleep_mock,
    ):
        cdp_wait_for_message.side_effect = [
            {"method": "Page.loadEventFired"},
            TimeoutError("timed out"),
        ]

        result = launcher_common._run_browser_smoke_check_via_cdp(
            "http://localhost:3000",
            chrome_process=mock.Mock(),
            profile_dir=Path("/tmp/profile"),
            timeout_seconds=0.1,
        )

        self.assertEqual(result.status, "passed")
        self.assertEqual(result.detail, "")
        wait_for_page_target.assert_called_once()
        evaluate_browser_expression.assert_called()

    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.02, 0.04])
    def test_wait_for_devtools_target_reads_profile_endpoint_for_launched_browser(self, _monotonic_mock, _sleep_mock):
        profile_dir = Path("/tmp/profile")
        process = mock.Mock()
        process.poll.return_value = None

        with mock.patch("launcher_common.Path.read_text", return_value="43123\n/devtools/browser/abc\n"):
            with mock.patch("launcher_common.Path.is_file", return_value=True):
                target = launcher_common._wait_for_devtools_target(profile_dir, chrome_process=process, timeout_seconds=0.1)

        self.assertEqual(target.port, 43123)
        self.assertEqual(target.websocket_url, "ws://127.0.0.1:43123/devtools/browser/abc")

    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.01, 0.02, 0.03, 0.05, 0.06])
    @mock.patch(
        "launcher_common._create_page_target",
        return_value=launcher_common._DevToolsTarget(port=43123, websocket_url="ws://127.0.0.1:43123/devtools/page/created"),
    )
    @mock.patch(
        "launcher_common._read_json_url",
        return_value=[{"type": "service_worker", "webSocketDebuggerUrl": "ws://127.0.0.1:43123/devtools/worker/1"}],
    )
    def test_wait_for_page_target_creates_page_when_list_has_no_page_targets(
        self,
        read_json_url_mock,
        create_page_target_mock,
        _monotonic_mock,
        _sleep_mock,
    ):
        process = mock.Mock()
        process.poll.return_value = None

        target = launcher_common._wait_for_page_target(43123, chrome_process=process, timeout_seconds=0.1)

        read_json_url_mock.assert_called_once()
        self.assertEqual(read_json_url_mock.call_args.args[0], "http://127.0.0.1:43123/json/list")
        self.assertLessEqual(read_json_url_mock.call_args.kwargs["timeout_seconds"], 0.1)
        create_page_target_mock.assert_called_once()
        self.assertEqual(create_page_target_mock.call_args.args[0], 43123)
        self.assertLessEqual(create_page_target_mock.call_args.kwargs["timeout_seconds"], 0.1)
        self.assertEqual(target.websocket_url, "ws://127.0.0.1:43123/devtools/page/created")

    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.01, 0.02, 0.03])
    @mock.patch(
        "launcher_common._read_json_url",
        return_value=[{"type": "page", "webSocketDebuggerUrl": "ws://127.0.0.1:43123/devtools/page/77"}],
    )
    def test_wait_for_page_target_prefers_page_websocket_over_browser_websocket(
        self,
        read_json_url_mock,
        _monotonic_mock,
        _sleep_mock,
    ):
        process = mock.Mock()
        process.poll.return_value = None

        target = launcher_common._wait_for_page_target(43123, chrome_process=process, timeout_seconds=0.1)

        read_json_url_mock.assert_called_once()
        self.assertEqual(read_json_url_mock.call_args.args[0], "http://127.0.0.1:43123/json/list")
        self.assertLessEqual(read_json_url_mock.call_args.kwargs["timeout_seconds"], 0.1)
        self.assertEqual(target.websocket_url, "ws://127.0.0.1:43123/devtools/page/77")

    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.02, 0.04])
    def test_wait_for_devtools_target_reports_early_browser_exit(self, _monotonic_mock, _sleep_mock):
        process = mock.Mock()
        process.poll.return_value = 23

        result = launcher_common._run_browser_smoke_check_via_cdp(
            "http://localhost:3000",
            chrome_process=process,
            profile_dir=Path("/tmp/profile"),
            timeout_seconds=0.1,
        )

        self.assertEqual(result.status, "browser_error")
        self.assertIn("browser exited before the debugging endpoint became ready", result.detail)


if __name__ == "__main__":
    unittest.main()
