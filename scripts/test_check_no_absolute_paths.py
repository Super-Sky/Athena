import importlib.util
import pathlib
import sys
import tempfile
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("check_no_absolute_paths.py")
SPEC = importlib.util.spec_from_file_location("check_no_absolute_paths", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class CheckNoAbsolutePathsTests(unittest.TestCase):
    def test_should_detect_macos_home_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "doc.md"
            path.write_text("see /Users/alice/project/file.md\n", encoding="utf-8")
            matches = MODULE.find_matches(path)
        self.assertEqual(len(matches), 1)
        self.assertEqual(matches[0][0], 1)

    def test_should_detect_windows_home_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "doc.md"
            path.write_text(r"see C:\Users\alice\project\file.md" + "\n", encoding="utf-8")
            matches = MODULE.find_matches(path)
        self.assertEqual(len(matches), 1)

    def test_should_ignore_placeholder_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "doc.md"
            path.write_text(r"see <drive>:\Users\<name>\project" + "\n", encoding="utf-8")
            matches = MODULE.find_matches(path)
        self.assertEqual(matches, [])

    def test_should_skip_claude_worktrees_only(self):
        self.assertTrue(
            MODULE.should_skip(pathlib.Path(".claude/worktrees/agent/docs/plan.md"))
        )
        self.assertFalse(
            MODULE.should_skip(pathlib.Path(".claude/skills/example/SKILL.md"))
        )


if __name__ == "__main__":
    unittest.main()
