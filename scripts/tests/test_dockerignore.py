import unittest
from pathlib import Path


class DockerignoreTests(unittest.TestCase):
    def test_dockerignore_excludes_local_rebuild_state_from_build_context(self):
        dockerignore = (Path(__file__).resolve().parents[2] / ".dockerignore").read_text(encoding="utf-8")

        self.assertIn(".worktrees", dockerignore)
        self.assertIn("data", dockerignore)
        self.assertIn("data-prod", dockerignore)
        self.assertIn("logs", dockerignore)
        self.assertIn("scripts/.runtime", dockerignore)


if __name__ == "__main__":
    unittest.main()
