import unittest
from pathlib import Path


class DockerfileFrontendBuildTests(unittest.TestCase):
    def test_frontend_build_defaults_to_prebuilt_and_requires_explicit_override_for_in_container_build(self):
        dockerfile = (Path(__file__).resolve().parents[2] / 'Dockerfile').read_text(encoding='utf-8')

        self.assertIn('ARG WEB_DIST_STRATEGY=prebuilt', dockerfile)
        self.assertIn('ARG APP_VERSION=', dockerfile)
        self.assertIn('case "$WEB_DIST_STRATEGY" in', dockerfile)
        self.assertIn('build)', dockerfile)
        self.assertIn('prebuilt)', dockerfile)
        self.assertNotIn('auto)', dockerfile)
        self.assertIn('$app_dir/dist is empty; use WEB_DIST_STRATEGY=build or provide a prebuilt dist', dockerfile)
        self.assertIn('copy_prebuilt_dist web/default /build/default-dist', dockerfile)
        self.assertIn('copy_prebuilt_dist web/classic /build/classic-dist', dockerfile)
        self.assertIn('validate_dist_integrity()', dockerfile)
        self.assertIn('missing asset referenced by index.html', dockerfile)
        self.assertIn('ARG WEB_BUILD_NODE_OPTIONS=--max-old-space-size=4096', dockerfile)
        self.assertIn('version="${APP_VERSION:-dev}"', dockerfile)


if __name__ == '__main__':
    unittest.main()
