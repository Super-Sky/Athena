import importlib.util
import os
import pathlib
import sys
import unittest
from unittest import mock


SCRIPT_PATH = pathlib.Path(__file__).with_name("fetch_gitlab_issue.py")
SPEC = importlib.util.spec_from_file_location("fetch_gitlab_issue", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class FetchGitLabIssueTests(unittest.TestCase):
    def test_parse_issue_url(self):
        base_url, project_path, issue_iid = MODULE.parse_issue_ref(
            "https://git.example.com/example-org/athena/-/issues/7"
        )
        self.assertEqual(base_url, "https://git.example.com")
        self.assertEqual(project_path, "example-org/athena")
        self.assertEqual(issue_iid, "7")

    def test_parse_project_ref(self):
        base_url, project_path, issue_iid = MODULE.parse_issue_ref("example-org/athena#7")
        self.assertIsNone(base_url)
        self.assertEqual(project_path, "example-org/athena")
        self.assertEqual(issue_iid, "7")

    def test_build_issue_api_url(self):
        url = MODULE.build_issue_api_url("https://git.example.com", "example-org/athena", "7")
        self.assertEqual(
            url,
            "https://git.example.com/api/v4/projects/new-world%2Fathena/issues/7",
        )

    def test_resolve_auth_prefers_gitlab_token(self):
        with mock.patch.dict(
            os.environ,
            {
                "GITLAB_TOKEN": "token-a",
                "GITLAB_PRIVATE_TOKEN": "token-b",
                "GITLAB_BEARER_TOKEN": "token-c",
            },
            clear=False,
        ):
            auth = MODULE.resolve_auth_headers()
        self.assertEqual(auth.source, "gitlab-token")
        self.assertEqual(auth.headers["PRIVATE-TOKEN"], "token-a")

    def test_normalize_issue(self):
        payload = MODULE.normalize_issue(
            {
                "iid": 3,
                "title": "Test issue",
                "description": "Issue description",
                "state": "opened",
                "task_status": {"count": 1, "completed_count": 0},
                "labels": ["backend", "athena"],
                "web_url": "https://git.example.com/example-org/athena/-/issues/7",
                "author": {"username": "alice"},
                "assignees": [{"username": "bob"}],
                "references": {"full": "example-org/athena#7", "short": "#7"},
            },
            source_ref="example-org/athena#7",
            project_path="example-org/athena",
            auth_source="private-token",
            notes=[{"author": "bot", "created_at": "now", "body": "hello", "system": False}],
        )
        self.assertEqual(payload["title"], "Test issue")
        self.assertEqual(payload["project_path"], "example-org/athena")
        self.assertEqual(payload["assignees"], ["bob"])
        self.assertEqual(payload["source_ref"], "example-org/athena#7")

    def test_render_markdown(self):
        text = MODULE.render_markdown(
            {
                "title": "Issue title",
                "source_ref": "example-org/athena#1",
                "auth_source": "glab",
                "project_path": "example-org/athena",
                "issue_iid": 1,
                "state": "opened",
                "labels": ["a", "b"],
                "assignees": ["dev"],
                "description": "body",
                "notes": [],
            }
        )
        self.assertIn("# Issue title", text)
        self.assertIn("## Description", text)
        self.assertIn("Source ref: example-org/athena#1", text)

    def test_fetch_issue_uses_glab_when_available(self):
        with mock.patch.object(MODULE, "glab_available", return_value=True), mock.patch.object(
            MODULE,
            "run_glab",
            return_value=(
                {
                    "iid": 3,
                    "title": "Issue title",
                    "description": "body",
                    "state": "opened",
                },
                None,
            ),
        ):
            payload = MODULE.fetch_issue("example-org/athena#7", include_notes=False)
        self.assertEqual(payload["auth_source"], "glab")
        self.assertEqual(payload["project_path"], "example-org/athena")


if __name__ == "__main__":
    unittest.main()
