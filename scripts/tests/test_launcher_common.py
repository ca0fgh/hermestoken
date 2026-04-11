import io
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock
from urllib.error import HTTPError, URLError

import sys

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import launcher_common


class _FakeResponse:
    def __init__(self, status=200):
        self.status = status

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False


class LauncherCommonTests(unittest.TestCase):
    def test_load_launcher_config_success(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            expected = {
                "compose_file": "docker-compose.yml",
                "local_url": "http://localhost:3000",
                "public_url": "https://pay-local.hermestoken.top",
                "cloudflared_tunnel_name": "hermestoken-local",
                "cloudflared_config_path": "~/.cloudflared/config.yml",
                "cloudflared_tunnel_token": "token-value",
                "cloudflared_tunnel_token_path": "~/.cloudflared/hermestoken-local.token",
                "healthcheck_timeout_seconds": 20,
                "healthcheck_interval_seconds": 1,
            }
            config_path.write_text(json.dumps(expected), encoding="utf-8")

            loaded = launcher_common.load_launcher_config(config_path)

        self.assertEqual(loaded.local_url, expected["local_url"])
        self.assertEqual(loaded.cloudflared_tunnel_name, expected["cloudflared_tunnel_name"])
        self.assertEqual(loaded.cloudflared_tunnel_token, expected["cloudflared_tunnel_token"])
        self.assertEqual(loaded.cloudflared_tunnel_token_path, expected["cloudflared_tunnel_token_path"])
        self.assertEqual(loaded.healthcheck_timeout_seconds, expected["healthcheck_timeout_seconds"])

    def test_load_launcher_config_treats_blank_optional_token_as_none(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text(
                json.dumps(
                    {
                        "compose_file": "docker-compose.yml",
                        "local_url": "http://localhost:3000",
                        "public_url": "https://pay-local.hermestoken.top",
                        "cloudflared_tunnel_name": "hermestoken-local",
                        "cloudflared_config_path": "~/.cloudflared/config.yml",
                        "cloudflared_tunnel_token": "   ",
                        "cloudflared_tunnel_token_path": "   ",
                        "healthcheck_timeout_seconds": 20,
                        "healthcheck_interval_seconds": 1,
                    }
                ),
                encoding="utf-8",
            )

            loaded = launcher_common.load_launcher_config(config_path)

        self.assertIsNone(loaded.cloudflared_tunnel_token)
        self.assertIsNone(loaded.cloudflared_tunnel_token_path)

    def test_load_launcher_config_rejects_non_string_optional_token(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text(
                json.dumps(
                    {
                        "compose_file": "docker-compose.yml",
                        "local_url": "http://localhost:3000",
                        "public_url": "https://pay-local.hermestoken.top",
                        "cloudflared_tunnel_name": "hermestoken-local",
                        "cloudflared_config_path": "~/.cloudflared/config.yml",
                        "cloudflared_tunnel_token": 123,
                        "cloudflared_tunnel_token_path": 456,
                        "healthcheck_timeout_seconds": 20,
                        "healthcheck_interval_seconds": 1,
                    }
                ),
                encoding="utf-8",
            )

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.load_launcher_config(config_path)

        self.assertIn("cloudflared_tunnel_token", str(ctx.exception))

    def test_load_launcher_config_rejects_non_string_optional_token_path(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text(
                json.dumps(
                    {
                        "compose_file": "docker-compose.yml",
                        "local_url": "http://localhost:3000",
                        "public_url": "https://pay-local.hermestoken.top",
                        "cloudflared_tunnel_name": "hermestoken-local",
                        "cloudflared_config_path": "~/.cloudflared/config.yml",
                        "cloudflared_tunnel_token_path": 123,
                        "healthcheck_timeout_seconds": 20,
                        "healthcheck_interval_seconds": 1,
                    }
                ),
                encoding="utf-8",
            )

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.load_launcher_config(config_path)

        self.assertIn("cloudflared_tunnel_token_path", str(ctx.exception))

    def test_load_launcher_config_missing_file_has_actionable_message(self):
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.load_launcher_config(Path("/no/such/launcher_config.json"))

        self.assertIn("launcher_config.json", str(ctx.exception))
        self.assertIn("Create", str(ctx.exception))

    def test_load_launcher_config_malformed_json(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text("{not-json", encoding="utf-8")

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.load_launcher_config(config_path)

        self.assertIn("Invalid JSON", str(ctx.exception))

    def test_load_launcher_config_rejects_non_object(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text(json.dumps(["wrong"]), encoding="utf-8")

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.load_launcher_config(config_path)

        self.assertIn("JSON object", str(ctx.exception))

    def test_load_launcher_config_rejects_invalid_field_type(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text(
                json.dumps(
                    {
                        "compose_file": "docker-compose.yml",
                        "local_url": "http://localhost:3000",
                        "public_url": "https://pay-local.hermestoken.top",
                        "cloudflared_tunnel_name": "hermestoken-local",
                        "cloudflared_config_path": "~/.cloudflared/config.yml",
                        "healthcheck_timeout_seconds": "20",
                        "healthcheck_interval_seconds": 1,
                    }
                ),
                encoding="utf-8",
            )

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.load_launcher_config(config_path)

        self.assertIn("healthcheck_timeout_seconds", str(ctx.exception))

    def test_load_launcher_config_rejects_boolean_timeout(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text(
                json.dumps(
                    {
                        "compose_file": "docker-compose.yml",
                        "local_url": "http://localhost:3000",
                        "public_url": "https://pay-local.hermestoken.top",
                        "cloudflared_tunnel_name": "hermestoken-local",
                        "cloudflared_config_path": "~/.cloudflared/config.yml",
                        "healthcheck_timeout_seconds": True,
                        "healthcheck_interval_seconds": 1,
                    }
                ),
                encoding="utf-8",
            )

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.load_launcher_config(config_path)

        self.assertIn("healthcheck_timeout_seconds", str(ctx.exception))

    def test_load_launcher_config_rejects_boolean_interval(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = Path(tmpdir) / "launcher_config.json"
            config_path.write_text(
                json.dumps(
                    {
                        "compose_file": "docker-compose.yml",
                        "local_url": "http://localhost:3000",
                        "public_url": "https://pay-local.hermestoken.top",
                        "cloudflared_tunnel_name": "hermestoken-local",
                        "cloudflared_config_path": "~/.cloudflared/config.yml",
                        "healthcheck_timeout_seconds": 20,
                        "healthcheck_interval_seconds": False,
                    }
                ),
                encoding="utf-8",
            )

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.load_launcher_config(config_path)

        self.assertIn("healthcheck_interval_seconds", str(ctx.exception))

    @mock.patch("launcher_common.shutil.which", return_value=None)
    def test_require_executable_missing_has_install_hint(self, _which):
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.require_executable("cloudflared", install_hint="Install cloudflared first.")

        self.assertIn("cloudflared", str(ctx.exception))
        self.assertIn("Install cloudflared first.", str(ctx.exception))

    @mock.patch("launcher_common.run_command")
    @mock.patch("launcher_common.require_executable")
    def test_require_docker_and_compose_checks_compose_subcommand(self, require_executable, run_command):
        require_executable.return_value = "/usr/bin/docker"
        run_command.return_value = subprocess.CompletedProcess(args=["docker", "compose", "version"], returncode=0)

        launcher_common.require_docker_and_compose()

        require_executable.assert_called_once_with(
            "docker",
            install_hint=mock.ANY,
        )
        run_command.assert_called_once_with(["docker", "compose", "version"], check=True, stream_output=False)

    @mock.patch("launcher_common.subprocess.run")
    def test_run_command_non_streamed(self, run_mock):
        run_mock.return_value = subprocess.CompletedProcess(args=["docker", "version"], returncode=0, stdout="ok", stderr="")

        completed = launcher_common.run_command(["docker", "version"], stream_output=False)

        self.assertEqual(completed.stdout, "ok")
        run_mock.assert_called_once()

    @mock.patch("launcher_common.subprocess.run")
    @mock.patch.dict(os.environ, {"PATH": "/usr/bin"}, clear=True)
    def test_run_command_non_streamed_merges_env_and_forwards_cwd(self, run_mock):
        run_mock.return_value = subprocess.CompletedProcess(args=["docker", "version"], returncode=0, stdout="ok", stderr="")

        launcher_common.run_command(
            ["docker", "version"],
            stream_output=False,
            cwd=Path("/tmp/test-cwd"),
            env={"CUSTOM_FLAG": "1"},
        )

        _, kwargs = run_mock.call_args
        self.assertEqual(kwargs["cwd"], "/tmp/test-cwd")
        self.assertIn("PATH", kwargs["env"])
        self.assertEqual(kwargs["env"]["PATH"], "/usr/bin")
        self.assertEqual(kwargs["env"]["CUSTOM_FLAG"], "1")

    @mock.patch("launcher_common.subprocess.Popen")
    def test_run_command_streamed(self, popen_mock):
        process = mock.Mock()
        process.stdout = ["line-1\n", "line-2\n"]
        process.wait.return_value = 0
        process.returncode = 0
        popen_mock.return_value = process

        output = io.StringIO()
        completed = launcher_common.run_command(["echo", "hi"], stream_output=True, stdout_stream=output)

        self.assertEqual(completed.returncode, 0)
        self.assertIn("line-1", output.getvalue())
        self.assertIn("line-2", output.getvalue())

    @mock.patch("launcher_common.subprocess.Popen")
    def test_run_command_streamed_failure_includes_output_context(self, popen_mock):
        process = mock.Mock()
        process.stdout = ["booting\n", "fatal: boom\n"]
        process.wait.return_value = 2
        process.returncode = 2
        popen_mock.return_value = process

        sink = io.StringIO()
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.run_command(["echo", "hi"], stream_output=True, stdout_stream=sink)

        self.assertIn("fatal: boom", str(ctx.exception))

    @mock.patch("launcher_common.urllib.request.urlopen")
    @mock.patch("launcher_common.time.monotonic")
    def test_poll_http_until_healthy_retries_then_succeeds(self, monotonic_mock, urlopen_mock):
        monotonic_mock.side_effect = [0.0, 0.0, 0.1, 0.2]
        urlopen_mock.side_effect = [URLError("offline"), _FakeResponse(status=200)]

        launcher_common.poll_http_until_healthy("http://localhost:3000", timeout_seconds=2, interval_seconds=0)

    @mock.patch("launcher_common.urllib.request.urlopen")
    @mock.patch("launcher_common.time.monotonic")
    def test_poll_http_until_healthy_sets_user_agent_header(self, monotonic_mock, urlopen_mock):
        monotonic_mock.side_effect = [0.0, 0.0, 0.1]
        captured = {}

        def _urlopen(request, timeout):
            captured["request"] = request
            captured["timeout"] = timeout
            return _FakeResponse(status=200)

        urlopen_mock.side_effect = _urlopen

        launcher_common.poll_http_until_healthy("https://pay-local.hermestoken.top", timeout_seconds=2, interval_seconds=0)

        request = captured["request"]
        self.assertEqual(request.get_method(), "GET")
        self.assertEqual(request.headers.get("User-agent"), launcher_common.DEFAULT_HEALTHCHECK_USER_AGENT)
        self.assertEqual(request.headers.get("Accept"), "*/*")

    @mock.patch("launcher_common.urllib.request.urlopen", side_effect=URLError("offline"))
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.1, 0.2, 1.1, 1.2])
    def test_poll_http_until_healthy_times_out(self, _monotonic, _urlopen):
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.poll_http_until_healthy("http://localhost:3000", timeout_seconds=1, interval_seconds=0)

        self.assertIn("http://localhost:3000", str(ctx.exception))
        self.assertIn("timed out", str(ctx.exception))

    @mock.patch(
        "launcher_common.urllib.request.urlopen",
        side_effect=HTTPError(
            url="http://localhost:3000",
            code=401,
            msg="Unauthorized",
            hdrs=None,
            fp=None,
        ),
    )
    @mock.patch("launcher_common.time.monotonic", side_effect=[0.0, 0.0, 0.1, 0.2, 1.1, 1.2])
    def test_poll_http_until_healthy_401_not_healthy_by_default(self, _monotonic, urlopen_mock):
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.poll_http_until_healthy("http://localhost:3000", timeout_seconds=1, interval_seconds=0)

        self.assertIn("timed out", str(ctx.exception))
        self.assertIn("unexpected HTTP status: 401", str(ctx.exception))
        self.assertGreaterEqual(urlopen_mock.call_count, 2)

    @mock.patch("launcher_common.time.monotonic", side_effect=[100.0, 100.2, 100.35, 100.7, 100.71])
    @mock.patch("launcher_common.time.sleep")
    @mock.patch("launcher_common.urllib.request.urlopen", side_effect=URLError("offline"))
    def test_poll_http_until_healthy_uses_remaining_time_for_request_timeout(self, urlopen_mock, _sleep_mock, _monotonic):
        with self.assertRaises(launcher_common.LauncherError):
            launcher_common.poll_http_until_healthy("http://localhost:3000", timeout_seconds=0.6, interval_seconds=0.1)

        first_call_timeout = urlopen_mock.call_args_list[0].kwargs["timeout"]
        self.assertLessEqual(first_call_timeout, 0.6)
        self.assertGreater(first_call_timeout, 0)

    @mock.patch("launcher_common.time.monotonic", side_effect=[5.0, 6.1])
    @mock.patch("launcher_common.urllib.request.urlopen")
    def test_poll_http_until_healthy_deadline_overrun_fails_without_request(self, urlopen_mock, _monotonic):
        with self.assertRaises(launcher_common.LauncherError):
            launcher_common.poll_http_until_healthy("http://localhost:3000", timeout_seconds=1, interval_seconds=0.1)
        urlopen_mock.assert_not_called()

    @mock.patch("launcher_common.subprocess.run")
    def test_validate_cfargotunnel_cname_success(self, run_mock):
        run_mock.return_value = subprocess.CompletedProcess(
            args=["nslookup", "-type=CNAME", "pay-local.hermestoken.top"],
            returncode=0,
            stdout="pay-local.hermestoken.top\tcanonical name = demo.cfargotunnel.com.\n",
            stderr="",
        )

        cname = launcher_common.validate_cfargotunnel_cname("pay-local.hermestoken.top")

        self.assertEqual(cname, "demo.cfargotunnel.com")

    @mock.patch("launcher_common.subprocess.run")
    def test_validate_cfargotunnel_cname_requires_cf_suffix(self, run_mock):
        run_mock.return_value = subprocess.CompletedProcess(
            args=["nslookup", "-type=CNAME", "pay-local.hermestoken.top"],
            returncode=0,
            stdout="pay-local.hermestoken.top canonical name = bad.example.com.\n",
            stderr="",
        )

        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.validate_cfargotunnel_cname("pay-local.hermestoken.top")

        self.assertIn(".cfargotunnel.com", str(ctx.exception))

    @mock.patch("launcher_common.subprocess.run", side_effect=FileNotFoundError("nslookup missing"))
    def test_validate_cfargotunnel_cname_handles_missing_nslookup(self, _run_mock):
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.validate_cfargotunnel_cname("pay-local.hermestoken.top")
        self.assertIn("nslookup", str(ctx.exception))

    @mock.patch("launcher_common.subprocess.run")
    def test_validate_cfargotunnel_cname_handles_non_zero_exit(self, run_mock):
        run_mock.return_value = subprocess.CompletedProcess(
            args=["nslookup", "-type=CNAME", "pay-local.hermestoken.top"],
            returncode=1,
            stdout="",
            stderr="SERVFAIL",
        )
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.validate_cfargotunnel_cname("pay-local.hermestoken.top")
        self.assertIn("SERVFAIL", str(ctx.exception))

    @mock.patch("launcher_common.subprocess.run")
    def test_validate_cfargotunnel_cname_handles_no_cname_answer(self, run_mock):
        run_mock.return_value = subprocess.CompletedProcess(
            args=["nslookup", "-type=CNAME", "pay-local.hermestoken.top"],
            returncode=0,
            stdout="Non-authoritative answer:\nName: pay-local.hermestoken.top\nAddress: 1.2.3.4\n",
            stderr="",
        )
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.validate_cfargotunnel_cname("pay-local.hermestoken.top")
        self.assertIn("No CNAME", str(ctx.exception))

    @mock.patch("launcher_common.subprocess.run", side_effect=subprocess.TimeoutExpired(cmd="nslookup", timeout=5))
    def test_validate_cfargotunnel_cname_handles_timeout(self, _run_mock):
        with self.assertRaises(launcher_common.LauncherError) as ctx:
            launcher_common.validate_cfargotunnel_cname("pay-local.hermestoken.top", lookup_timeout_seconds=5)
        self.assertIn("timed out", str(ctx.exception))

    @mock.patch("launcher_common.subprocess.run")
    def test_validate_cfargotunnel_cname_supports_alias_output_format(self, run_mock):
        run_mock.return_value = subprocess.CompletedProcess(
            args=["nslookup", "-type=CNAME", "pay-local.hermestoken.top"],
            returncode=0,
            stdout="pay-local.hermestoken.top is an alias for demo.cfargotunnel.com.\n",
            stderr="",
        )
        cname = launcher_common.validate_cfargotunnel_cname("pay-local.hermestoken.top")
        self.assertEqual(cname, "demo.cfargotunnel.com")


if __name__ == "__main__":
    unittest.main()
