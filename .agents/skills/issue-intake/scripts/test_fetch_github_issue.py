import importlib.util
import os
import pathlib
import sys
import unittest
from unittest import mock


SCRIPT_PATH = pathlib.Path(__file__).with_name("fetch_github_issue.py")
SPEC = importlib.util.spec_from_file_location("fetch_github_issue", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class FetchGitHubIssueTests(unittest.TestCase):
    def test_parse_issue_url(self):
        base_url, project_path, issue_number = MODULE.parse_issue_ref(
            "https://github.com/Super-Sky/Athena/issues/7"
        )
        self.assertEqual(base_url, "https://github.com")
        self.assertEqual(project_path, "Super-Sky/Athena")
        self.assertEqual(issue_number, "7")

    def test_parse_project_ref(self):
        base_url, project_path, issue_number = MODULE.parse_issue_ref("Super-Sky/Athena#7")
        self.assertIsNone(base_url)
        self.assertEqual(project_path, "Super-Sky/Athena")
        self.assertEqual(issue_number, "7")

    def test_build_issue_api_url(self):
        url = MODULE.build_issue_api_url("https://api.github.com", "Super-Sky/Athena", "7")
        self.assertEqual(
            url,
            "https://api.github.com/repos/Super-Sky/Athena/issues/7",
        )

    def test_resolve_auth_prefers_github_token(self):
        with mock.patch.dict(
            os.environ,
            {
                "GITHUB_TOKEN": "token-a",
                "GH_TOKEN": "token-b",
                "GITHUB_BEARER_TOKEN": "token-c",
            },
            clear=False,
        ):
            auth = MODULE.resolve_auth_headers()
        self.assertEqual(auth.source, "github_token")
        self.assertEqual(auth.headers["Authorization"], "Bearer token-a")

    def test_normalize_issue(self):
        payload = MODULE.normalize_issue(
            {
                "number": 3,
                "title": "Test issue",
                "body": "Issue description",
                "state": "open",
                "labels": [{"name": "backend"}, {"name": "athena"}],
                "html_url": "https://github.com/Super-Sky/Athena/issues/7",
                "user": {"login": "alice"},
                "assignees": [{"login": "bob"}],
            },
            source_ref="Super-Sky/Athena#7",
            project_path="Super-Sky/Athena",
            auth_source="github_token",
            comments=[{"author": "bot", "created_at": "now", "body": "hello", "system": False}],
        )
        self.assertEqual(payload["title"], "Test issue")
        self.assertEqual(payload["project_path"], "Super-Sky/Athena")
        self.assertEqual(payload["assignees"], ["bob"])
        self.assertEqual(payload["source_ref"], "Super-Sky/Athena#7")

    def test_render_markdown(self):
        text = MODULE.render_markdown(
            {
                "title": "Issue title",
                "source_ref": "Super-Sky/Athena#1",
                "auth_source": "gh",
                "project_path": "Super-Sky/Athena",
                "issue_number": 1,
                "state": "open",
                "labels": ["a", "b"],
                "assignees": ["dev"],
                "description": "body",
                "comments": [],
            }
        )
        self.assertIn("# Issue title", text)
        self.assertIn("## Description", text)
        self.assertIn("Source ref: Super-Sky/Athena#1", text)

    def test_fetch_issue_uses_gh_when_available(self):
        with mock.patch.object(MODULE, "gh_available", return_value=True), mock.patch.object(
            MODULE,
            "run_gh",
            return_value=(
                {
                    "number": 3,
                    "title": "Issue title",
                    "body": "body",
                    "state": "open",
                },
                None,
            ),
        ):
            payload = MODULE.fetch_issue("Super-Sky/Athena#7", include_comments=False)
        self.assertEqual(payload["auth_source"], "gh")
        self.assertEqual(payload["project_path"], "Super-Sky/Athena")


if __name__ == "__main__":
    unittest.main()
