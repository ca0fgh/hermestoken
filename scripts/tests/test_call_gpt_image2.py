import base64
import io
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
TOOL_DIR = SCRIPTS_DIR / "tool"
if str(TOOL_DIR) not in sys.path:
    sys.path.insert(0, str(TOOL_DIR))

import call_gpt_image2


class CallGptImage2Tests(unittest.TestCase):
    def test_submit_url_adds_default_images_generation_path(self):
        url = call_gpt_image2.build_submit_url("https://hermestoken.top/")

        self.assertEqual(url, "https://hermestoken.top/v1/images/generations")

    def test_default_output_is_next_to_script(self):
        self.assertEqual(Path(call_gpt_image2.DEFAULT_OUTPUT).parent, TOOL_DIR)
        self.assertEqual(Path(call_gpt_image2.DEFAULT_OUTPUT).name, "gpt_image2_output.png")

    def test_make_request_payload_uses_gpt_image2_defaults(self):
        args = call_gpt_image2.apply_default_args(
            mock.Mock(
                api_key="sk-secret-value",
                model=None,
                prompt="draw a small robot",
                size=None,
                quality=None,
                n=None,
                response_format=None,
                output_format=None,
                output=None,
                request_timeout=None,
                dry_run=True,
            )
        )

        payload = call_gpt_image2.make_request_payload(args)

        self.assertEqual(payload["model"], "openai/gpt-image-2")
        self.assertEqual(payload["prompt"], "draw a small robot")
        self.assertEqual(payload["size"], "1024x1024")
        self.assertEqual(payload["quality"], "auto")
        self.assertEqual(payload["n"], 1)
        self.assertEqual(payload["response_format"], "b64_json")
        self.assertEqual(payload["output_format"], "png")

    def test_extract_image_items_supports_common_response_shapes(self):
        items = call_gpt_image2.extract_image_items(
            {
                "data": [
                    {"url": "https://cdn.example/image.png"},
                    {"b64_json": "ZmFrZS1pbWFnZQ=="},
                ]
            }
        )

        self.assertEqual(
            items,
            [
                {"url": "https://cdn.example/image.png"},
                {"b64_json": "ZmFrZS1pbWFnZQ=="},
            ],
        )

    def test_save_base64_image_writes_decoded_file(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output_path = Path(tmpdir) / "image.png"

            saved = call_gpt_image2.save_base64_image(
                base64.b64encode(b"fake-image").decode("ascii"),
                output_path,
            )

            self.assertEqual(saved, output_path)
            self.assertEqual(output_path.read_bytes(), b"fake-image")

    def test_main_dry_run_prints_request_without_network_or_secret(self):
        stdout = io.StringIO()
        stderr = io.StringIO()

        exit_code = call_gpt_image2.main(
            [
                "--api-key",
                "sk-secret-value",
                "--base-url",
                "https://example.invalid",
                "--prompt",
                "test prompt",
                "--dry-run",
            ],
            stdout=stdout,
            stderr=stderr,
        )

        self.assertEqual(exit_code, 0)
        output = stdout.getvalue()
        self.assertIn("POST https://hermestoken.top/v1/images/generations", output)
        self.assertIn('"model": "openai/gpt-image-2"', output)
        self.assertIn('"prompt": "test prompt"', output)
        self.assertIn("Authorization: Bearer sk-...alue", output)
        self.assertNotIn("sk-secret-value", output)
        self.assertNotIn("example.invalid", output)

    @mock.patch("call_gpt_image2.getpass.getpass", return_value="sk-secret-value")
    def test_main_interactive_dry_run_prompts_for_options(self, getpass_mock):
        stdout = io.StringIO()
        stderr = io.StringIO()
        stdin = io.StringIO("1\ninteractive prompt\n3\n4\n2\n1\n3\n\n30\ny\n")

        exit_code = call_gpt_image2.main(
            ["--interactive"],
            stdout=stdout,
            stderr=stderr,
            stdin=stdin,
        )

        self.assertEqual(exit_code, 0)
        getpass_mock.assert_called_once()
        prompts = stderr.getvalue()
        self.assertIn("模型", prompts)
        self.assertIn("openai/gpt-image-2", prompts)
        self.assertIn("尺寸", prompts)
        self.assertIn("质量", prompts)
        output = stdout.getvalue()
        self.assertIn('"model": "openai/gpt-image-2"', output)
        self.assertIn('"prompt": "interactive prompt"', output)
        self.assertIn('"size": "1536x1024"', output)
        self.assertIn('"quality": "high"', output)
        self.assertIn('"n": 2', output)
        self.assertIn('"response_format": "b64_json"', output)
        self.assertIn('"output_format": "webp"', output)
        self.assertNotIn("sk-secret-value", output)

    @mock.patch("call_gpt_image2.http_request_json")
    def test_main_saves_base64_response(self, http_request_json):
        encoded = base64.b64encode(b"fake-image").decode("ascii")
        http_request_json.return_value = {"data": [{"b64_json": encoded}]}
        stdout = io.StringIO()
        stderr = io.StringIO()

        with tempfile.TemporaryDirectory() as tmpdir:
            output_path = Path(tmpdir) / "out.png"
            exit_code = call_gpt_image2.main(
                [
                    "--api-key",
                    "sk-secret-value",
                    "--prompt",
                    "test prompt",
                    "--output",
                    str(output_path),
                ],
                stdout=stdout,
                stderr=stderr,
            )

            self.assertEqual(exit_code, 0)
            self.assertEqual(output_path.read_bytes(), b"fake-image")
        called_url, called_method = http_request_json.call_args.args[:2]
        self.assertEqual(called_url, "https://hermestoken.top/v1/images/generations")
        self.assertEqual(called_method, "POST")
        self.assertIn("[ok] saved=", stdout.getvalue())


if __name__ == "__main__":
    unittest.main()
