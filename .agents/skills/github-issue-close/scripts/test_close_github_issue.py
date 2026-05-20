import importlib.util
import json
import pathlib
import sys
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("close_github_issue.py")
SPEC = importlib.util.spec_from_file_location("close_github_issue", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class CloseGitHubIssueTests(unittest.TestCase):
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

    def test_build_close_note(self):
        note = MODULE.build_close_note(
            branch="feat/issue-7",
            commit="abc123",
            pr_url="https://github.com/Super-Sky/Athena/pull/1",
            issue_requirements=["issue 要求 A"],
            reconciled=["要求 A -> PR 1 + go test"],
            completed=["完成 A"],
            verification=["go test ./..."],
            remaining=[],
            decision="关闭：scope 已完成",
        )
        self.assertIn("Issue closeout reconciliation completed", note)
        self.assertIn("issue 要求 A", note)
        self.assertIn("要求 A -> PR 1 + go test", note)
        self.assertIn("`feat/issue-7`", note)
        self.assertIn("完成 A", note)
        self.assertIn("go test ./...", note)
        self.assertIn("关闭：scope 已完成", note)

    def test_close_not_allowed_with_remaining_work(self):
        self.assertFalse(MODULE.close_allowed(remaining=["等待联调"], allow_incomplete=False))
        self.assertTrue(MODULE.close_allowed(remaining=["等待联调"], allow_incomplete=True))

    def test_preview_json_cli(self):
        code = MODULE.main([
            "--issue",
            "Super-Sky/Athena#7",
            "--issue-requirement",
            "issue 要求 A",
            "--reconciled",
            "要求 A -> PR 1 + go test",
            "--completed",
            "完成 A",
            "--verification",
            "go test ./...",
            "--decision",
            "关闭：scope 已完成",
            "--json",
        ])
        self.assertEqual(code, 0)

    def test_refuse_close_with_remaining_work(self):
        code = MODULE.main([
            "--issue",
            "Super-Sky/Athena#7",
            "--remaining",
            "等待联调",
            "--decision",
            "不能关闭",
            "--close",
        ])
        self.assertEqual(code, 1)

    def test_refuse_close_without_reconciliation(self):
        code = MODULE.main([
            "--issue",
            "Super-Sky/Athena#7",
            "--completed",
            "完成 A",
            "--verification",
            "go test ./...",
            "--decision",
            "关闭：scope 已完成",
            "--close",
        ])
        self.assertEqual(code, 1)


if __name__ == "__main__":
    unittest.main()
