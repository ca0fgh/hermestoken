import io
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

SCRIPTS_DIR = Path(__file__).resolve().parents[1]
TOOL_DIR = SCRIPTS_DIR / "tool"
if str(TOOL_DIR) not in sys.path:
    sys.path.insert(0, str(TOOL_DIR))

import call_veo31


class CallVeo31Tests(unittest.TestCase):
    def test_submit_url_adds_default_video_generation_path(self):
        url = call_veo31.build_submit_url("https://hermestoken.top/")

        self.assertEqual(url, "https://hermestoken.top/v1/video/generations")

    def test_submit_url_preserves_explicit_endpoint(self):
        url = call_veo31.build_submit_url("https://hermestoken.top/base", "/v1/videos")

        self.assertEqual(url, "https://hermestoken.top/base/v1/videos")

    def test_poll_url_uses_matching_submit_family(self):
        self.assertEqual(
            call_veo31.build_poll_url("https://hermestoken.top", "/v1/videos", "video_123"),
            "https://hermestoken.top/v1/videos/video_123",
        )
        self.assertEqual(
            call_veo31.build_poll_url("https://hermestoken.top", "/v1/video/generations", "task_123"),
            "https://hermestoken.top/v1/video/generations/task_123",
        )

    def test_build_multipart_includes_required_veo_fields_without_api_key(self):
        body, content_type = call_veo31.build_multipart_form_data(
            fields={
                "model": call_veo31.DEFAULT_MODEL,
                "prompt": "a small robot painting a sunrise",
                "size": "720x1280",
                "seconds": "8",
                "enable_upsample": "false",
            },
            files=[],
            boundary="test-boundary",
        )

        text = body.decode("utf-8")
        self.assertEqual(content_type, "multipart/form-data; boundary=test-boundary")
        self.assertIn('name="model"', text)
        self.assertIn("veo_3_1", text)
        self.assertNotIn("veo_3_1-fast", text)
        self.assertIn('name="prompt"', text)
        self.assertIn("a small robot painting a sunrise", text)
        self.assertNotIn("sk-secret", text)

    def test_extract_task_id_supports_common_response_shapes(self):
        self.assertEqual(call_veo31.extract_task_id({"id": "video_1"}), "video_1")
        self.assertEqual(call_veo31.extract_task_id({"data": {"task_id": "task_2"}}), "task_2")
        self.assertEqual(call_veo31.extract_task_id({"result": {"video_id": "video_3"}}), "video_3")

    def test_extract_video_url_supports_nested_arrays(self):
        self.assertEqual(
            call_veo31.extract_video_url({"data": {"videos": [{"url": "https://cdn/video.mp4"}]}}),
            "https://cdn/video.mp4",
        )
        self.assertEqual(
            call_veo31.extract_video_url({"output": [{"download_url": "https://cdn/out.mp4"}]}),
            "https://cdn/out.mp4",
        )

    def test_http_request_json_uses_requests_when_available(self):
        response = mock.Mock()
        response.status_code = 200
        response.content = b'{"ok": true}'
        response.json.return_value = {"ok": True}
        response.text = '{"ok": true}'
        fake_requests = mock.Mock()
        fake_requests.request.return_value = response
        fake_requests.RequestException = Exception

        with mock.patch.object(call_veo31, "requests", fake_requests):
            payload = call_veo31.http_request_json(
                "https://hermestoken.top/v1/video/generations",
                "POST",
                {"Content-Type": "multipart/form-data"},
                b"body",
                timeout=12,
            )

        self.assertEqual(payload, {"ok": True})
        fake_requests.request.assert_called_once_with(
            "POST",
            "https://hermestoken.top/v1/video/generations",
            headers={"Content-Type": "multipart/form-data"},
            data=b"body",
            timeout=12,
        )

    def test_download_video_uses_requests_without_auth_for_external_url(self):
        response = mock.Mock()
        response.status_code = 200
        response.content = b"fake-video"
        response.text = ""
        fake_requests = mock.Mock()
        fake_requests.get.return_value = response
        fake_requests.RequestException = Exception

        with tempfile.TemporaryDirectory() as tmpdir:
            output_path = Path(tmpdir) / "out.mp4"
            with (
                mock.patch.object(call_veo31, "requests", fake_requests),
                mock.patch.object(
                    call_veo31.request,
                    "urlopen",
                    side_effect=AssertionError("urlopen should not be used when requests is available"),
                ),
            ):
                destination = call_veo31.download_video(
                    video_url="https://cdn.example/video.mp4",
                    base_url="https://hermestoken.top",
                    api_key="sk-secret-value",
                    output_path=str(output_path),
                    timeout=12,
                )

            self.assertEqual(destination, output_path)
            self.assertEqual(output_path.read_bytes(), b"fake-video")
        fake_requests.get.assert_called_once()
        called_url = fake_requests.get.call_args.args[0]
        called_headers = fake_requests.get.call_args.kwargs["headers"]
        self.assertEqual(called_url, "https://cdn.example/video.mp4")
        self.assertEqual(fake_requests.get.call_args.kwargs["timeout"], 12)
        self.assertNotIn("Authorization", called_headers)

    def test_main_dry_run_prints_request_without_network_or_secret(self):
        stdout = io.StringIO()
        stderr = io.StringIO()
        argv = [
            "--api-key",
            "sk-secret-value",
            "--base-url",
            "https://hermestoken.top/",
            "--prompt",
            "test prompt",
            "--dry-run",
        ]

        exit_code = call_veo31.main(argv, stdout=stdout, stderr=stderr)

        self.assertEqual(exit_code, 0)
        output = stdout.getvalue()
        self.assertIn("POST https://hermestoken.top/v1/video/generations", output)
        self.assertIn("veo_3_1", output)
        self.assertNotIn("veo_3_1-fast", output)
        self.assertIn("test prompt", output)
        self.assertNotIn("sk-secret-value", output)
        self.assertIn("Authorization: Bearer sk-...alue", output)

    def test_main_dry_run_uses_fixed_url_even_when_legacy_base_url_is_passed(self):
        stdout = io.StringIO()
        stderr = io.StringIO()

        exit_code = call_veo31.main(
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
        self.assertIn("POST https://hermestoken.top/v1/video/generations", output)
        self.assertNotIn("example.invalid", output)

    @mock.patch("call_veo31.getpass.getpass", return_value="sk-secret-value")
    def test_main_interactive_dry_run_prompts_for_api_key_and_request_fields(self, getpass_mock):
        stdout = io.StringIO()
        stderr = io.StringIO()
        stdin = io.StringIO("\ninteractive prompt\n2\n6\n\ny\ny\n")

        exit_code = call_veo31.main(
            ["--interactive"],
            stdout=stdout,
            stderr=stderr,
            stdin=stdin,
        )

        self.assertEqual(exit_code, 0)
        getpass_mock.assert_called_once()
        prompts = stderr.getvalue()
        self.assertIn("模型", prompts)
        self.assertIn("1. veo_3_1", prompts)
        self.assertIn("2. veo_3_1-4K", prompts)
        output = stdout.getvalue()
        self.assertIn("POST https://hermestoken.top/v1/video/generations", output)
        self.assertIn("interactive prompt", output)
        self.assertIn('"size": "1280x720"', output)
        self.assertIn('"seconds": "6"', output)
        self.assertIn('"enable_upsample": "true"', output)
        self.assertNotIn("sk-secret-value", output)

    @mock.patch("call_veo31.getpass.getpass", return_value="sk-secret-value")
    def test_main_interactive_can_select_4k_model_by_number(self, getpass_mock):
        stdout = io.StringIO()
        stderr = io.StringIO()
        stdin = io.StringIO("2\n4k prompt\n1\n8\n\nn\ny\n")

        exit_code = call_veo31.main(
            ["--interactive"],
            stdout=stdout,
            stderr=stderr,
            stdin=stdin,
        )

        self.assertEqual(exit_code, 0)
        getpass_mock.assert_called_once()
        output = stdout.getvalue()
        self.assertIn('"model": "veo_3_1-4K"', output)
        self.assertIn('"size": "720x1280"', output)

    @mock.patch("call_veo31.http_request_json")
    def test_submit_only_posts_form_and_returns_task(self, http_request_json):
        http_request_json.return_value = {"id": "task_123", "status": "queued"}
        stdout = io.StringIO()
        stderr = io.StringIO()

        exit_code = call_veo31.main(
            [
                "--api-key",
                "sk-secret-value",
                "--base-url",
                "https://hermestoken.top",
                "--prompt",
                "test prompt",
                "--no-poll",
            ],
            stdout=stdout,
            stderr=stderr,
        )

        self.assertEqual(exit_code, 0)
        called_url, called_method = http_request_json.call_args.args[:2]
        called_headers = http_request_json.call_args.args[2]
        called_body = http_request_json.call_args.args[3]
        self.assertEqual(called_url, "https://hermestoken.top/v1/video/generations")
        self.assertEqual(called_method, "POST")
        self.assertEqual(called_headers["Authorization"], "Bearer sk-secret-value")
        self.assertIn(b"test prompt", called_body)
        self.assertIn("task_123", stdout.getvalue())

    @mock.patch("call_veo31.download_video")
    @mock.patch("call_veo31.http_request_json")
    def test_default_download_output_is_next_to_script(self, http_request_json, download_video):
        http_request_json.return_value = {"url": "https://cdn.example/video.mp4"}
        download_video.return_value = Path(call_veo31.DEFAULT_OUTPUT)
        stdout = io.StringIO()
        stderr = io.StringIO()

        exit_code = call_veo31.main(
            [
                "--api-key",
                "sk-secret-value",
                "--prompt",
                "test prompt",
            ],
            stdout=stdout,
            stderr=stderr,
        )

        self.assertEqual(exit_code, 0)
        self.assertEqual(Path(call_veo31.DEFAULT_OUTPUT).parent, TOOL_DIR)
        self.assertEqual(download_video.call_args.kwargs["output_path"], call_veo31.DEFAULT_OUTPUT)


if __name__ == "__main__":
    unittest.main()
