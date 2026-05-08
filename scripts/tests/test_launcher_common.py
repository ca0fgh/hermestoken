import io
import json
import os
import ssl
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

    @mock.patch.dict(os.environ, {}, clear=True)
    def test_resolve_web_dist_strategy_defaults_to_prebuilt(self):
        self.assertEqual(launcher_common.resolve_web_dist_strategy(), "prebuilt")

    @mock.patch.dict(os.environ, {"WEB_DIST_STRATEGY": "build"}, clear=True)
    def test_resolve_web_dist_strategy_rejects_docker_side_frontend_build_modes(self):
        with self.assertRaises(launcher_common.LauncherError) as context:
            launcher_common.resolve_web_dist_strategy()

        self.assertIn("avoid Docker-side Vite OOM", str(context.exception))

    @mock.patch("launcher_common.run_command")
    def test_resolve_application_version_prefers_version_file(self, run_command):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            (repo_root / "VERSION").write_text("1.2.3\n", encoding="utf-8")

            version = launcher_common.resolve_application_version(repo_root=repo_root)

        self.assertEqual(version, "1.2.3")
        run_command.assert_not_called()

    @mock.patch("launcher_common.run_command")
    def test_resolve_application_version_falls_back_to_git_describe_when_version_file_empty(self, run_command):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            (repo_root / "VERSION").write_text("", encoding="utf-8")
            run_command.return_value = subprocess.CompletedProcess(
                args=["git", "describe", "--tags", "--always", "--dirty"],
                returncode=0,
                stdout="e3f7bef8-dirty\n",
                stderr="",
            )

            version = launcher_common.resolve_application_version(repo_root=repo_root)

        self.assertEqual(version, "e3f7bef8-dirty")
        run_command.assert_called_once_with(
            ["git", "describe", "--tags", "--always", "--dirty"],
            check=False,
            stream_output=False,
            cwd=repo_root,
        )

    @mock.patch.dict(os.environ, {}, clear=True)
    @mock.patch("launcher_common.run_command")
    @mock.patch("launcher_common.require_executable")
    @mock.patch("launcher_common.validate_frontend_dist_integrity")
    def test_prepare_frontend_dist_for_docker_packaging_builds_on_host(
        self,
        validate_frontend_dist_integrity,
        require_executable,
        run_command,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            web_dir = repo_root / "web"
            web_dir.mkdir()
            default_dir = web_dir / "default"
            classic_dir = web_dir / "classic"
            default_dir.mkdir()
            classic_dir.mkdir()
            (default_dir / "package.json").write_text("{}", encoding="utf-8")
            (classic_dir / "package.json").write_text("{}", encoding="utf-8")
            (repo_root / "VERSION").write_text("1.2.3\n", encoding="utf-8")
            stdout = io.StringIO()

            launcher_common.prepare_frontend_dist_for_docker_packaging(output=stdout, repo_root=repo_root)

        require_executable.assert_called_once_with("bun", install_hint=mock.ANY)
        run_command.assert_has_calls(
            [
                mock.call(
                    ["bun", "install"],
                    check=True,
                    stream_output=True,
                    cwd=default_dir,
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "run", "build"],
                    check=True,
                    stream_output=True,
                    cwd=default_dir,
                    env={
                        "DISABLE_ESLINT_PLUGIN": "true",
                        "NODE_OPTIONS": "--max-old-space-size=4096",
                        "VITE_REACT_APP_VERSION": "1.2.3",
                    },
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "install"],
                    check=True,
                    stream_output=True,
                    cwd=classic_dir,
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "run", "build"],
                    check=True,
                    stream_output=True,
                    cwd=classic_dir,
                    env={
                        "DISABLE_ESLINT_PLUGIN": "true",
                        "NODE_OPTIONS": "--max-old-space-size=4096",
                        "VITE_REACT_APP_VERSION": "1.2.3",
                    },
                    stdout_stream=stdout,
                ),
            ]
        )
        validate_frontend_dist_integrity.assert_has_calls(
            [
                mock.call(default_dir / "dist"),
                mock.call(classic_dir / "dist"),
            ]
        )
        self.assertIn("WEB_DIST_STRATEGY=prebuilt", stdout.getvalue())

    @mock.patch.dict(os.environ, {}, clear=True)
    @mock.patch("launcher_common.run_command")
    @mock.patch("launcher_common.require_executable")
    @mock.patch("launcher_common.validate_frontend_dist_integrity")
    def test_prepare_frontend_dist_for_docker_packaging_forwards_asset_base_url(
        self,
        validate_frontend_dist_integrity,
        require_executable,
        run_command,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            web_dir = repo_root / "web"
            web_dir.mkdir()
            default_dir = web_dir / "default"
            classic_dir = web_dir / "classic"
            default_dir.mkdir()
            classic_dir.mkdir()
            (default_dir / "package.json").write_text("{}", encoding="utf-8")
            (classic_dir / "package.json").write_text("{}", encoding="utf-8")
            (repo_root / "VERSION").write_text("1.2.3\n", encoding="utf-8")
            stdout = io.StringIO()

            launcher_common.prepare_frontend_dist_for_docker_packaging(
                output=stdout,
                repo_root=repo_root,
                env={"VITE_ASSET_BASE_URL": "https://static.hermestoken.top"},
            )

        require_executable.assert_called_once_with("bun", install_hint=mock.ANY)
        run_command.assert_has_calls(
            [
                mock.call(
                    ["bun", "install"],
                    check=True,
                    stream_output=True,
                    cwd=default_dir,
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "run", "build"],
                    check=True,
                    stream_output=True,
                    cwd=default_dir,
                    env={
                        "DISABLE_ESLINT_PLUGIN": "true",
                        "NODE_OPTIONS": "--max-old-space-size=4096",
                        "VITE_ASSET_BASE_URL": "https://static.hermestoken.top/",
                        "VITE_REACT_APP_VERSION": "1.2.3",
                    },
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "install"],
                    check=True,
                    stream_output=True,
                    cwd=classic_dir,
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "run", "build"],
                    check=True,
                    stream_output=True,
                    cwd=classic_dir,
                    env={
                        "DISABLE_ESLINT_PLUGIN": "true",
                        "NODE_OPTIONS": "--max-old-space-size=4096",
                        "VITE_ASSET_BASE_URL": "https://static.hermestoken.top/",
                        "VITE_REACT_APP_VERSION": "1.2.3",
                    },
                    stdout_stream=stdout,
                ),
            ]
        )
        validate_frontend_dist_integrity.assert_has_calls(
            [
                mock.call(default_dir / "dist"),
                mock.call(classic_dir / "dist"),
            ]
        )
        self.assertIn("Asset base URL", stdout.getvalue())

    @mock.patch.dict(os.environ, {}, clear=True)
    @mock.patch("launcher_common.run_command")
    @mock.patch("launcher_common.require_executable")
    @mock.patch("launcher_common.validate_frontend_dist_integrity")
    def test_prepare_frontend_dist_for_docker_packaging_falls_back_to_git_describe_when_version_file_empty(
        self,
        validate_frontend_dist_integrity,
        require_executable,
        run_command,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            web_dir = repo_root / "web"
            web_dir.mkdir()
            default_dir = web_dir / "default"
            classic_dir = web_dir / "classic"
            default_dir.mkdir()
            classic_dir.mkdir()
            (default_dir / "package.json").write_text("{}", encoding="utf-8")
            (classic_dir / "package.json").write_text("{}", encoding="utf-8")
            (repo_root / "VERSION").write_text("", encoding="utf-8")
            stdout = io.StringIO()
            run_command.side_effect = [
                subprocess.CompletedProcess(
                    args=["git", "describe", "--tags", "--always", "--dirty"],
                    returncode=0,
                    stdout="e3f7bef8\n",
                    stderr="",
                ),
                subprocess.CompletedProcess(args=["bun", "install"], returncode=0, stdout="", stderr=""),
                subprocess.CompletedProcess(args=["bun", "run", "build"], returncode=0, stdout="", stderr=""),
                subprocess.CompletedProcess(args=["bun", "install"], returncode=0, stdout="", stderr=""),
                subprocess.CompletedProcess(args=["bun", "run", "build"], returncode=0, stdout="", stderr=""),
            ]

            launcher_common.prepare_frontend_dist_for_docker_packaging(output=stdout, repo_root=repo_root)

        require_executable.assert_called_once_with("bun", install_hint=mock.ANY)
        run_command.assert_has_calls(
            [
                mock.call(
                    ["git", "describe", "--tags", "--always", "--dirty"],
                    check=False,
                    stream_output=False,
                    cwd=repo_root,
                ),
                mock.call(
                    ["bun", "install"],
                    check=True,
                    stream_output=True,
                    cwd=default_dir,
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "run", "build"],
                    check=True,
                    stream_output=True,
                    cwd=default_dir,
                    env={
                        "DISABLE_ESLINT_PLUGIN": "true",
                        "NODE_OPTIONS": "--max-old-space-size=4096",
                        "VITE_REACT_APP_VERSION": "e3f7bef8",
                    },
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "install"],
                    check=True,
                    stream_output=True,
                    cwd=classic_dir,
                    stdout_stream=stdout,
                ),
                mock.call(
                    ["bun", "run", "build"],
                    check=True,
                    stream_output=True,
                    cwd=classic_dir,
                    env={
                        "DISABLE_ESLINT_PLUGIN": "true",
                        "NODE_OPTIONS": "--max-old-space-size=4096",
                        "VITE_REACT_APP_VERSION": "e3f7bef8",
                    },
                    stdout_stream=stdout,
                ),
            ]
        )
        validate_frontend_dist_integrity.assert_has_calls(
            [
                mock.call(default_dir / "dist"),
                mock.call(classic_dir / "dist"),
            ]
        )
        self.assertIn("git describe fallback", stdout.getvalue())

    @mock.patch.dict(os.environ, {}, clear=True)
    @mock.patch("launcher_common.require_executable")
    def test_prepare_frontend_dist_for_docker_packaging_requires_dual_frontend_packages(
        self,
        require_executable,
    ):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            web_dir = repo_root / "web"
            (web_dir / "default").mkdir(parents=True)
            (web_dir / "classic").mkdir()
            (web_dir / "default" / "package.json").write_text("{}", encoding="utf-8")
            (repo_root / "VERSION").write_text("1.2.3\n", encoding="utf-8")

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                launcher_common.prepare_frontend_dist_for_docker_packaging(
                    output=io.StringIO(),
                    repo_root=repo_root,
                )

        require_executable.assert_called_once_with("bun", install_hint=mock.ANY)
        self.assertIn("web/classic", str(ctx.exception))

    def test_validate_frontend_dist_integrity_accepts_index_references_that_exist(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            dist_dir = Path(tmpdir)
            assets_dir = dist_dir / "assets"
            assets_dir.mkdir()
            (dist_dir / "index.html").write_text(
                """
                <!doctype html>
                <html>
                  <head>
                    <link rel="stylesheet" href="/assets/index-abc123.css" />
                  </head>
                  <body>
                    <script type="module" src="/assets/index-xyz789.js"></script>
                  </body>
                </html>
                """,
                encoding="utf-8",
            )
            (assets_dir / "index-abc123.css").write_text("body{}", encoding="utf-8")
            (assets_dir / "index-xyz789.js").write_text("console.log('ok')", encoding="utf-8")

            launcher_common.validate_frontend_dist_integrity(dist_dir)

    def test_validate_frontend_dist_integrity_accepts_absolute_cdn_asset_references(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            dist_dir = Path(tmpdir)
            assets_dir = dist_dir / "assets"
            assets_dir.mkdir()
            (dist_dir / "index.html").write_text(
                """
                <!doctype html>
                <html>
                  <head>
                    <script type="module" src="https://static.hermestoken.top/assets/index-xyz789.js"></script>
                  </head>
                  <body>
                    <link rel="modulepreload" href="https://static.hermestoken.top/assets/react-core-abc123.js" />
                  </body>
                </html>
                """,
                encoding="utf-8",
            )
            (assets_dir / "index-xyz789.js").write_text("console.log('ok')", encoding="utf-8")
            (assets_dir / "react-core-abc123.js").write_text("console.log('core')", encoding="utf-8")

            launcher_common.validate_frontend_dist_integrity(dist_dir)

    def test_validate_frontend_dist_integrity_rejects_missing_asset_reference(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            dist_dir = Path(tmpdir)
            assets_dir = dist_dir / "assets"
            assets_dir.mkdir()
            (dist_dir / "index.html").write_text(
                """
                <!doctype html>
                <html>
                  <body>
                    <script type="module" src="/assets/index-missing.js"></script>
                  </body>
                </html>
                """,
                encoding="utf-8",
            )

            with self.assertRaises(launcher_common.LauncherError) as context:
                launcher_common.validate_frontend_dist_integrity(dist_dir)

        self.assertIn("/assets/index-missing.js", str(context.exception))
        self.assertIn("stay in sync", str(context.exception))

    def test_validate_frontend_dist_integrity_rejects_missing_index_html(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            dist_dir = Path(tmpdir)

            with self.assertRaises(launcher_common.LauncherError) as context:
                launcher_common.validate_frontend_dist_integrity(dist_dir)

        self.assertIn("index.html", str(context.exception))

    @mock.patch("launcher_common.run_command")
    def test_remove_legacy_compose_containers_removes_only_matching_legacy_project_for_target_compose(self, run_command):
        compose_file_path = Path("/repo/docker-compose.yml")
        repo_root = Path("/repo")
        output = io.StringIO()
        run_command.side_effect = [
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="hermestoken|/repo/docker-compose.yml\n", stderr=""),
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="", stderr=""),
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="hermestoken|/repo/docker-compose.prod.yml\n", stderr=""),
            subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="hermestoken-local|/repo/docker-compose.yml\n", stderr=""),
        ]

        removed = launcher_common.remove_legacy_compose_containers(
            legacy_project_name="hermestoken",
            compose_file_path=compose_file_path,
            container_names=("hermestoken", "redis", "postgres"),
            output=output,
            repo_root=repo_root,
        )

        self.assertEqual(removed, ["hermestoken"])
        run_command.assert_has_calls(
            [
                mock.call(
                    [
                        "docker",
                        "inspect",
                        "-f",
                        '{{ index .Config.Labels "com.docker.compose.project" }}|{{ index .Config.Labels "com.docker.compose.project.config_files" }}',
                        "hermestoken",
                    ],
                    check=False,
                    stream_output=False,
                    cwd=repo_root,
                ),
                mock.call(
                    ["docker", "rm", "-f", "hermestoken"],
                    check=True,
                    stream_output=False,
                    cwd=repo_root,
                ),
                mock.call(
                    [
                        "docker",
                        "inspect",
                        "-f",
                        '{{ index .Config.Labels "com.docker.compose.project" }}|{{ index .Config.Labels "com.docker.compose.project.config_files" }}',
                        "redis",
                    ],
                    check=False,
                    stream_output=False,
                    cwd=repo_root,
                ),
                mock.call(
                    [
                        "docker",
                        "inspect",
                        "-f",
                        '{{ index .Config.Labels "com.docker.compose.project" }}|{{ index .Config.Labels "com.docker.compose.project.config_files" }}',
                        "postgres",
                    ],
                    check=False,
                    stream_output=False,
                    cwd=repo_root,
                ),
            ]
        )
        self.assertIn("Removed legacy compose containers: hermestoken", output.getvalue())

    @mock.patch("launcher_common.run_command")
    def test_remove_legacy_compose_containers_skips_missing_containers(self, run_command):
        run_command.return_value = subprocess.CompletedProcess(args=["docker"], returncode=1, stdout="", stderr="missing")

        removed = launcher_common.remove_legacy_compose_containers(
            legacy_project_name="hermestoken",
            compose_file_path=Path("/repo/docker-compose.yml"),
            container_names=("hermestoken",),
            output=io.StringIO(),
            repo_root=Path("/repo"),
        )

        self.assertEqual(removed, [])
        run_command.assert_called_once()

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

    @mock.patch("launcher_common.run_command")
    def test_ensure_named_docker_volume_creates_volume(self, run_command):
        repo_root = Path("/repo")
        output = io.StringIO()

        launcher_common.ensure_named_docker_volume("hermestoken_pg_data", output=output, repo_root=repo_root)

        run_command.assert_called_once_with(
            ["docker", "volume", "create", "hermestoken_pg_data"],
            check=True,
            stream_output=False,
            cwd=repo_root,
        )
        self.assertIn("Docker volume ready: hermestoken_pg_data", output.getvalue())

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
    def test_poll_http_until_healthy_retries_on_connection_reset(self, monotonic_mock, urlopen_mock):
        monotonic_mock.side_effect = [0.0, 0.0, 0.1, 0.2]
        urlopen_mock.side_effect = [ConnectionResetError("connection reset by peer"), _FakeResponse(status=200)]

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

    @mock.patch("launcher_common.os.path.isfile")
    @mock.patch("launcher_common.ssl.create_default_context")
    @mock.patch("launcher_common.urllib.request.urlopen")
    @mock.patch("launcher_common.time.monotonic")
    def test_poll_http_until_healthy_retries_https_with_system_ca_bundle_after_cert_verify_failure(
        self,
        monotonic_mock,
        urlopen_mock,
        create_default_context_mock,
        isfile_mock,
    ):
        monotonic_mock.side_effect = [0.0, 0.0, 0.1, 0.2, 0.3, 0.4, 2.1]
        isfile_mock.side_effect = lambda path: path == "/etc/ssl/cert.pem"
        fallback_context = object()
        create_default_context_mock.return_value = fallback_context

        cert_error = ssl.SSLCertVerificationError(1, "certificate verify failed: unable to get local issuer certificate")

        def _urlopen(request, timeout, context=None):
            if context is fallback_context:
                return _FakeResponse(status=200)
            raise URLError(cert_error)

        urlopen_mock.side_effect = _urlopen

        launcher_common.poll_http_until_healthy(
            "https://pay-local.hermestoken.top",
            timeout_seconds=2,
            interval_seconds=0,
        )

        create_default_context_mock.assert_called_once_with(cafile="/etc/ssl/cert.pem")
        self.assertEqual(urlopen_mock.call_count, 2)
        self.assertNotIn("context", urlopen_mock.call_args_list[0].kwargs)
        self.assertIs(urlopen_mock.call_args_list[1].kwargs["context"], fallback_context)

    @mock.patch("launcher_common.os.path.isfile")
    @mock.patch("launcher_common.ssl.create_default_context")
    @mock.patch("launcher_common.urllib.request.urlopen")
    @mock.patch("launcher_common.time.monotonic")
    def test_poll_http_until_healthy_does_not_use_ca_fallback_for_http_urls(
        self,
        monotonic_mock,
        urlopen_mock,
        create_default_context_mock,
        isfile_mock,
    ):
        monotonic_mock.side_effect = [0.0, 0.0, 0.1]
        urlopen_mock.return_value = _FakeResponse(status=200)
        isfile_mock.return_value = True

        launcher_common.poll_http_until_healthy(
            "http://localhost:3000",
            timeout_seconds=2,
            interval_seconds=0,
        )

        create_default_context_mock.assert_not_called()
        self.assertEqual(urlopen_mock.call_count, 1)
        self.assertNotIn("context", urlopen_mock.call_args.kwargs)

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
