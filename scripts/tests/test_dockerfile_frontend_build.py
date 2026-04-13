import unittest
from pathlib import Path


class DockerfileFrontendBuildTests(unittest.TestCase):
    def test_frontend_build_defaults_to_fresh_build_with_explicit_prebuilt_opt_in(self):
        dockerfile = (Path(__file__).resolve().parents[2] / 'Dockerfile').read_text(encoding='utf-8')

        self.assertIn('ARG WEB_DIST_STRATEGY=build', dockerfile)
        self.assertIn('case "$WEB_DIST_STRATEGY" in', dockerfile)
        self.assertIn('prebuilt)', dockerfile)
        self.assertNotIn('if [ -d web/dist ] && [ -n "$(ls -A web/dist 2>/dev/null)" ]; then', dockerfile)


if __name__ == '__main__':
    unittest.main()
