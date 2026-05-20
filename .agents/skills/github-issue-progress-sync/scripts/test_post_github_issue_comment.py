import importlib.util
import pathlib
import sys
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("post_github_issue_comment.py")
SPEC = importlib.util.spec_from_file_location("post_github_issue_comment", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class PostGitHubIssueCommentTests(unittest.TestCase):
    def test_parse_issue_url(self):
        base, project, iid = MODULE.parse_issue_ref(
            "https://github.com/Super-Sky/Athena/issues/7"
        )
        self.assertEqual(base, "https://github.com")
        self.assertEqual(project, "Super-Sky/Athena")
        self.assertEqual(iid, "7")

    def test_parse_project_ref(self):
        base, project, iid = MODULE.parse_issue_ref("Super-Sky/Athena#7")
        self.assertIsNone(base)
        self.assertEqual(project, "Super-Sky/Athena")
        self.assertEqual(iid, "7")

    def test_split_items(self):
        items = MODULE.split_items(["a;b", " c ", "", "d"])
        self.assertEqual(items, ["a", "b", "c", "d"])

    def test_build_push_note(self):
        text = MODULE.build_note(
            event="push",
            branch="feat/issue-4",
            commit="abc123",
            pr_url=None,
            completed=["完成 A"],
            pending=["待处理 B"],
            next_steps=["发起 PR"],
        )
        self.assertIn("当前仓工作分支已推送", text)
        self.assertIn("`feat/issue-4`", text)
        self.assertIn("完成 A", text)
        self.assertIn("待处理 B", text)
        self.assertIn("发起 PR", text)

    def test_build_merge_note_with_pr(self):
        text = MODULE.build_note(
            event="merge",
            branch="feat/issue-4",
            commit="abc123",
            pr_url="https://github.com/Super-Sky/Athena/pull/1",
            completed=["完成 A"],
            pending=[],
            next_steps=[],
        )
        self.assertIn("当前仓变更已合入主线", text)
        self.assertIn("https://github.com/Super-Sky/Athena/pull/1", text)


if __name__ == "__main__":
    unittest.main()
