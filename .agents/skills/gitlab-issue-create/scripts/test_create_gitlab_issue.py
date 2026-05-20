import importlib.util
import json
import os
import pathlib
import sys
import unittest
from unittest import mock


SCRIPT_PATH = pathlib.Path(__file__).with_name("create_gitlab_issue.py")
SPEC = importlib.util.spec_from_file_location("create_gitlab_issue", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class CreateGitLabIssueTests(unittest.TestCase):
    def test_parse_project_url(self):
        ref = MODULE.parse_project_ref(project=None, project_url="https://git.example.com/example-org/athena")
        self.assertEqual(ref.base_url, "https://git.example.com")
        self.assertEqual(ref.project_path, "example-org/athena")

    def test_parse_project_path(self):
        ref = MODULE.parse_project_ref(project="example-org/athena", project_url=None)
        self.assertIsNone(ref.base_url)
        self.assertEqual(ref.project_path, "example-org/athena")

    def test_render_issue_body_checks_selected_fields(self):
        body = MODULE.render_issue_body(
            issue_type="需要增强现有能力",
            background="调试 skill 管理",
            problem="只能编辑轻量配置",
            request="支持完整包编辑",
            current_result="无法查看 revision",
            expected_result="可以查看并替换包",
            urgency="中",
            supplemental=["截图链接"],
            handling_mode="进入功能实现",
            canonical_issue="example-org/athena#3",
            primary_repository="athena",
            external_references=["optional design reference"],
            owner="maxt",
        )
        self.assertIn("- [x] 需要增强现有能力", body)
        self.assertIn("- [x] 中", body)
        self.assertIn("使用场景或业务背景：调试 skill 管理", body)
        self.assertIn("- [x] 进入功能实现", body)
        self.assertIn("canonical issue：example-org/athena#3", body)
        self.assertIn("- [x] athena", body)
        self.assertIn("optional design reference", body)
        self.assertIn("owner：maxt", body)

    def test_render_issue_body_rejects_invalid_urgency(self):
        with self.assertRaises(ValueError):
            MODULE.render_issue_body(
                issue_type="需要新增能力",
                background="",
                problem="",
                request="",
                current_result="",
                expected_result="",
                urgency="紧急",
                supplemental=[],
            )

    def test_split_items_supports_semicolons(self):
        self.assertEqual(MODULE.split_items(["a;b", " c ", ""]), ["a", "b", "c"])

    def test_preview_json_cli(self):
        with mock.patch.object(MODULE, "glab_available", return_value=False):
            captured: list[str] = []
            with mock.patch("builtins.print", side_effect=lambda text="", **kwargs: captured.append(str(text))):
                code = MODULE.main([
                    "--project",
                    "example-org/athena",
                    "--title",
                    "测试 issue",
                    "--issue-type",
                    "当前功能有问题",
                    "--problem",
                    "问题描述",
                    "--urgency",
                    "低",
                    "--json",
                ])
        self.assertEqual(code, 0)
        payload = json.loads("\n".join(captured))
        self.assertEqual(payload["project_path"], "example-org/athena")
        self.assertEqual(payload["title"], "测试 issue")
        self.assertIn("问题描述", payload["description"])

    def test_resolve_base_url_from_env(self):
        with mock.patch.dict(os.environ, {"GITLAB_BASE_URL": "https://git.example.com/"}, clear=False):
            self.assertEqual(MODULE.resolve_base_url(None), "https://git.example.com")


if __name__ == "__main__":
    unittest.main()
