import unittest
from pathlib import Path


class DockerfileFrontendBuildTests(unittest.TestCase):
    def test_frontend_build_defaults_to_prebuilt_and_requires_explicit_override_for_in_container_build(self):
        dockerfile = (Path(__file__).resolve().parents[2] / 'Dockerfile').read_text(encoding='utf-8')

        self.assertIn('ARG WEB_DIST_STRATEGY=prebuilt', dockerfile)
        self.assertIn('case "$WEB_DIST_STRATEGY" in', dockerfile)
        self.assertIn('build)', dockerfile)
        self.assertIn('prebuilt)', dockerfile)
        self.assertNotIn('auto)', dockerfile)
        self.assertIn('web/dist is empty; use WEB_DIST_STRATEGY=build or provide a prebuilt dist', dockerfile)
        self.assertIn('ARG WEB_BUILD_NODE_OPTIONS=--max-old-space-size=4096', dockerfile)


if __name__ == '__main__':
    unittest.main()
