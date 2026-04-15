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
    @mock.patch("public._start_tunnel_container")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.local.run_local_stack")
    def test_run_public_stack_reuses_local_stack_then_starts_dockerized_tunnel(
        self,
        run_local_stack,
        validate_cfargotunnel_cname,
        start_tunnel_container,
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
        start_tunnel_container.assert_called_once_with(
            config,
            cloudflared_token=None,
            cloudflared_config_path=cloudflared_config,
            repo_root=repo_root,
        )
        wait_for_public_readiness.assert_called_once_with(config, repo_root=repo_root)
        run_browser_smoke_check.assert_called_once_with("https://pay-local.hermestoken.top", output=stdout)
        self.assertIn("[ok] Public update healthy", stdout.getvalue())

    @mock.patch("public.run_command")
    def test_start_tunnel_container_uses_token_env_and_shares_new_api_network_namespace(self, run_command):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            token_path = repo_root / "hermestoken.token"
            token_path.write_text("token\n", encoding="utf-8")
            config = self._config("/unused/config.yml", cloudflared_tunnel_token_path=str(token_path))

            public._start_tunnel_container(
                config,
                cloudflared_token="token",
                cloudflared_config_path=None,
                repo_root=repo_root,
            )

        self.assertEqual(run_command.call_count, 2)
        self.assertEqual(
            run_command.call_args_list[0],
            mock.call(
                ["docker", "rm", "-f", public.PUBLIC_TUNNEL_CONTAINER_NAME],
                check=False,
                stream_output=False,
                cwd=repo_root,
            ),
        )
        docker_run_command = run_command.call_args_list[1].args[0]
        self.assertIn("--network", docker_run_command)
        self.assertIn(f"container:{public.LOCAL_APP_CONTAINER_NAME}", docker_run_command)
        self.assertIn("-e", docker_run_command)
        self.assertIn("TUNNEL_TOKEN=token", docker_run_command)
        self.assertIn(public.CLOUDFLARED_IMAGE, docker_run_command)

    @mock.patch("public.run_command")
    def test_start_tunnel_container_mounts_config_directory_for_locally_managed_tunnel(self, run_command):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            cloudflared_dir = repo_root / ".cloudflared"
            cloudflared_dir.mkdir()
            credentials = cloudflared_dir / "uuid.json"
            credentials.write_text("{}", encoding="utf-8")
            config_path = cloudflared_dir / "config.yml"
            config_path.write_text(
                "tunnel: hermestoken-local\ncredentials-file: {}\ningress:\n  - hostname: pay-local.hermestoken.top\n    service: http://localhost:3000\n".format(credentials),
                encoding="utf-8",
            )
            config = self._config(str(config_path))

            public._start_tunnel_container(
                config,
                cloudflared_token=None,
                cloudflared_config_path=config_path,
                repo_root=repo_root,
            )

        docker_run_command = run_command.call_args_list[1].args[0]
        self.assertIn(f"{cloudflared_dir}:{cloudflared_dir}:ro", docker_run_command)
        self.assertIn("--config", docker_run_command)
        self.assertIn(str(config_path), docker_run_command)
        self.assertIn("run", docker_run_command)
        self.assertIn("hermestoken-local", docker_run_command)

    @mock.patch("public._read_recent_tunnel_output", return_value=public.deque(["cloudflared booting", "origin refused"], maxlen=20))
    @mock.patch("public._stop_tunnel_container")
    @mock.patch(
        "public._wait_for_public_readiness",
        side_effect=launcher_common.LauncherError("Health check timed out for https://pay-local.hermestoken.top."),
    )
    @mock.patch("public._start_tunnel_container")
    @mock.patch("public.validate_cfargotunnel_cname")
    @mock.patch("public.local.run_local_stack")
    def test_run_public_stack_stops_tunnel_and_adds_log_context_when_public_health_fails(
        self,
        run_local_stack,
        validate_cfargotunnel_cname,
        start_tunnel_container,
        _wait_for_public_readiness,
        stop_tunnel_container,
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
        start_tunnel_container.assert_called_once()
        stop_tunnel_container.assert_called_once_with(repo_root=repo_root)
        self.assertIn("origin refused", str(context.exception))

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
