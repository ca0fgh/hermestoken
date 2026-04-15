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
import public


class PublicLauncherTests(unittest.TestCase):
    def _config(
        self,
        cloudflared_config_path: str,
        *,
        cloudflared_tunnel_token=None,
        cloudflared_tunnel_token_path=None,
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
            cloudflared_tunnel_token_path=cloudflared_tunnel_token_path,
            healthcheck_timeout_seconds=timeout_seconds,
            healthcheck_interval_seconds=interval_seconds,
        )

    @mock.patch("public.run_browser_smoke_check")
    @mock.patch("public._wait_for_public_readiness")
    @mock.patch("public._start_tunnel_process", return_value=4321)
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.local.run_local_stack")
    def test_run_public_stack_reuses_local_stack_then_starts_host_tunnel(
        self,
        run_local_stack,
        validate_cfargotunnel_cname,
        start_tunnel_process,
        wait_for_public_readiness,
        run_browser_smoke_check,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_config = repo_root / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            config = self._config(str(cloudflared_config))
            stdout = io.StringIO()

            public.run_public_stack(config, output=stdout, repo_root=repo_root, action_label="update")

        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        run_local_stack.assert_called_once_with(config, output=stdout, repo_root=repo_root, action_label="update")
        start_tunnel_process.assert_called_once_with(
            config,
            cloudflared_token=None,
            cloudflared_config_path=cloudflared_config,
            repo_root=repo_root,
        )
        wait_for_public_readiness.assert_called_once_with(config, repo_root=repo_root)
        run_browser_smoke_check.assert_called_once_with("https://pay-local.hermestoken.top", output=stdout)
        self.assertIn("[ok] Tunnel process started: 4321", stdout.getvalue())
        self.assertIn("[ok] Public update healthy", stdout.getvalue())

    @mock.patch("public.subprocess.Popen")
    @mock.patch("public._stop_tunnel_process")
    @mock.patch("public.require_cloudflared", return_value="/opt/homebrew/bin/cloudflared")
    def test_start_tunnel_process_uses_token_env_and_writes_pid_file(
        self,
        require_cloudflared,
        stop_tunnel_process,
        popen,
    ):
        process = mock.Mock()
        process.pid = 1234
        process.poll.return_value = None
        popen.return_value = process

        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            config = self._config("/unused/config.yml", cloudflared_tunnel_token="token")

            pid = public._start_tunnel_process(
                config,
                cloudflared_token="token",
                cloudflared_config_path=None,
                repo_root=repo_root,
            )

            pid_file = public._tunnel_pid_file_path(repo_root=repo_root)
            log_file = public._tunnel_log_file_path(repo_root=repo_root)
            pid_value = pid_file.read_text(encoding="utf-8").strip()
            log_exists = log_file.is_file()

        self.assertEqual(pid, 1234)
        require_cloudflared.assert_called_once_with()
        stop_tunnel_process.assert_called_once_with(repo_root=repo_root)
        command = popen.call_args.args[0]
        kwargs = popen.call_args.kwargs
        self.assertEqual(
            command,
            ["/opt/homebrew/bin/cloudflared", "tunnel", "--no-autoupdate", "run"],
        )
        self.assertEqual(kwargs["cwd"], str(repo_root))
        self.assertEqual(kwargs["env"]["TUNNEL_TOKEN"], "token")
        self.assertTrue(kwargs["start_new_session"])
        self.assertEqual(pid_value, "1234")
        self.assertTrue(log_exists)

    @mock.patch("public.subprocess.Popen")
    @mock.patch("public._stop_tunnel_process")
    @mock.patch("public.require_cloudflared", return_value="/opt/homebrew/bin/cloudflared")
    def test_start_tunnel_process_uses_config_file_for_locally_managed_tunnel(
        self,
        require_cloudflared,
        stop_tunnel_process,
        popen,
    ):
        process = mock.Mock()
        process.pid = 5678
        process.poll.return_value = None
        popen.return_value = process

        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_dir = repo_root / ".cloudflared"
            cloudflared_dir.mkdir()
            config_path = cloudflared_dir / "config.yml"
            config_path.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            config = self._config(str(config_path))

            public._start_tunnel_process(
                config,
                cloudflared_token=None,
                cloudflared_config_path=config_path,
                repo_root=repo_root,
            )

        require_cloudflared.assert_called_once_with()
        stop_tunnel_process.assert_called_once_with(repo_root=repo_root)
        command = popen.call_args.args[0]
        kwargs = popen.call_args.kwargs
        self.assertEqual(
            command,
            [
                "/opt/homebrew/bin/cloudflared",
                "tunnel",
                "--no-autoupdate",
                "--config",
                str(config_path),
                "run",
                "hermestoken-local",
            ],
        )
        self.assertNotIn("TUNNEL_TOKEN", kwargs["env"])

    @mock.patch("public.time.sleep")
    @mock.patch("public.os.kill")
    @mock.patch("public._cleanup_legacy_tunnel_container")
    @mock.patch("public._is_tracked_tunnel_process", return_value=True)
    @mock.patch("public._process_exists", side_effect=[True, False, False])
    def test_stop_tunnel_process_terminates_running_cloudflared_pid_and_clears_pid_file(
        self,
        _process_exists,
        _is_tracked_tunnel_process,
        cleanup_legacy_tunnel_container,
        os_kill,
        _sleep,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            pid_file = public._tunnel_pid_file_path(repo_root=repo_root)
            pid_file.parent.mkdir(parents=True, exist_ok=True)
            pid_file.write_text("2468\n", encoding="utf-8")

            public._stop_tunnel_process(repo_root=repo_root)

            self.assertFalse(pid_file.exists())

        os_kill.assert_called_once_with(2468, public.signal.SIGTERM)
        cleanup_legacy_tunnel_container.assert_called_once_with(repo_root=repo_root)

    @mock.patch("public._read_recent_tunnel_output", return_value=public.deque(["cloudflared booting", "origin refused"], maxlen=20))
    @mock.patch("public._stop_tunnel_process")
    @mock.patch(
        "public._wait_for_public_readiness",
        side_effect=launcher_common.LauncherError("Health check timed out for https://pay-local.hermestoken.top."),
    )
    @mock.patch("public._start_tunnel_process", return_value=4321)
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.local.run_local_stack")
    def test_run_public_stack_stops_tunnel_and_adds_log_context_when_public_health_fails(
        self,
        run_local_stack,
        validate_cfargotunnel_cname,
        start_tunnel_process,
        _wait_for_public_readiness,
        stop_tunnel_process,
        _read_recent_tunnel_output,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_config = repo_root / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            config = self._config(str(cloudflared_config))

            with self.assertRaises(launcher_common.LauncherError) as context:
                public.run_public_stack(config, output=io.StringIO(), repo_root=repo_root)

        run_local_stack.assert_called_once_with(config, output=mock.ANY, repo_root=repo_root, action_label="deploy")
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        start_tunnel_process.assert_called_once()
        stop_tunnel_process.assert_called_once_with(repo_root=repo_root)
        self.assertIn("origin refused", str(context.exception))

    @mock.patch(
        "public.run_browser_smoke_check",
        side_effect=launcher_common.LauncherError("Browser smoke check failed for https://pay-local.hermestoken.top: browser could not start."),
    )
    @mock.patch("public._wait_for_public_readiness")
    @mock.patch("public._stop_tunnel_process")
    @mock.patch("public._start_tunnel_process", return_value=4321)
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.local.run_local_stack")
    def test_run_public_stack_keeps_tunnel_running_when_browser_smoke_fails_after_public_health(
        self,
        run_local_stack,
        validate_cfargotunnel_cname,
        start_tunnel_process,
        stop_tunnel_process,
        wait_for_public_readiness,
        run_browser_smoke_check,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_config = repo_root / "config.yml"
            cloudflared_config.write_text("tunnel: hermestoken-local\n", encoding="utf-8")
            config = self._config(str(cloudflared_config))
            stdout = io.StringIO()

            public.run_public_stack(config, output=stdout, repo_root=repo_root)

        run_local_stack.assert_called_once_with(config, output=stdout, repo_root=repo_root, action_label="deploy")
        validate_cfargotunnel_cname.assert_called_once_with("pay-local.hermestoken.top")
        start_tunnel_process.assert_called_once()
        wait_for_public_readiness.assert_called_once_with(config, repo_root=repo_root)
        run_browser_smoke_check.assert_called_once_with("https://pay-local.hermestoken.top", output=stdout)
        stop_tunnel_process.assert_not_called()
        self.assertIn("[warn] Public browser smoke check failed after health check", stdout.getvalue())
        self.assertIn("[ok] Public deploy healthy", stdout.getvalue())

    @mock.patch("public.run_public_stack")
    @mock.patch("public.load_launcher_config")
    def test_main_accepts_update_command(self, load_config, run_public_stack):
        load_config.return_value = self._config("/tmp/config.yml")

        with mock.patch("sys.argv", ["public.py", "update"]):
            exit_code = public.main()

        self.assertEqual(exit_code, 0)
        run_public_stack.assert_called_once_with(load_config.return_value, output=sys.stdout, action_label="update")


if __name__ == "__main__":
    unittest.main()
