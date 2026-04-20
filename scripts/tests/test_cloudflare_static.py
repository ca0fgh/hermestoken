import io
import sys
import tempfile
import unittest
from datetime import date
from pathlib import Path
from unittest import mock

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import cloudflare_static
import launcher_common


class CloudflareStaticDeployTests(unittest.TestCase):
    def test_deploy_static_assets_requires_cloudflare_account_id(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            asset_dir = Path(tmp_dir)
            (asset_dir / "index.html").write_text("ok", encoding="utf-8")

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                cloudflare_static.deploy_static_assets(
                    worker_name="old-base-c009",
                    asset_dir=asset_dir,
                    env={"CLOUDFLARE_API_TOKEN": "token"},
                )

        self.assertIn("CLOUDFLARE_ACCOUNT_ID", str(ctx.exception))

    def test_deploy_static_assets_requires_cloudflare_api_token(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            asset_dir = Path(tmp_dir)
            (asset_dir / "index.html").write_text("ok", encoding="utf-8")

            with self.assertRaises(launcher_common.LauncherError) as ctx:
                cloudflare_static.deploy_static_assets(
                    worker_name="old-base-c009",
                    asset_dir=asset_dir,
                    env={"CLOUDFLARE_ACCOUNT_ID": "account"},
                )

        self.assertIn("CLOUDFLARE_API_TOKEN", str(ctx.exception))

    @mock.patch("cloudflare_static.run_command")
    @mock.patch("cloudflare_static.require_executable", return_value="/opt/homebrew/bin/bun")
    def test_deploy_static_assets_runs_wrangler_deploy_with_assets(
        self,
        require_executable,
        run_command,
    ):
        stdout = io.StringIO()
        with tempfile.TemporaryDirectory() as tmp_dir:
            asset_dir = Path(tmp_dir)
            (asset_dir / "index.html").write_text("ok", encoding="utf-8")

            cloudflare_static.deploy_static_assets(
                worker_name="old-base-c009",
                asset_dir=asset_dir,
                env={
                    "CLOUDFLARE_ACCOUNT_ID": "account-id",
                    "CLOUDFLARE_API_TOKEN": "token-value",
                },
                output=stdout,
                repo_root=Path("/repo"),
                compatibility_date="2026-04-20",
                message="release build",
            )

        require_executable.assert_called_once()
        run_command.assert_called_once_with(
            [
                "bunx",
                "wrangler",
                "deploy",
                "--name",
                "old-base-c009",
                "--compatibility-date",
                "2026-04-20",
                "--assets",
                str(asset_dir),
                "--message",
                "release build",
            ],
            check=True,
            stream_output=True,
            cwd=Path("/repo"),
            env={
                "CLOUDFLARE_ACCOUNT_ID": "account-id",
                "CLOUDFLARE_API_TOKEN": "token-value",
                "CI": "1",
            },
            stdout_stream=stdout,
        )
        self.assertIn("Cloudflare static assets deployed", stdout.getvalue())

    @mock.patch("cloudflare_static.deploy_static_assets")
    def test_main_deploy_invokes_cloudflare_static_release(
        self,
        deploy_static_assets,
    ):
        stdout = io.StringIO()
        with tempfile.TemporaryDirectory() as tmp_dir, mock.patch(
            "sys.argv",
            [
                "cloudflare_static.py",
                "deploy",
                "--worker-name",
                "old-base-c009",
                "--asset-dir",
                tmp_dir,
            ],
        ), mock.patch("sys.stdout", stdout):
            exit_code = cloudflare_static.main()

        self.assertEqual(exit_code, 0)
        deploy_static_assets.assert_called_once_with(
            worker_name="old-base-c009",
            asset_dir=Path(tmp_dir),
            env=mock.ANY,
            output=stdout,
            repo_root=cloudflare_static.REPO_ROOT,
            compatibility_date=date.today().isoformat(),
            message="",
        )


if __name__ == "__main__":
    unittest.main()
