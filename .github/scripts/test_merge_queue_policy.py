"""Tests for the dev merge queue policy check."""

from contextlib import redirect_stderr, redirect_stdout
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("merge_queue_policy.py")
SPEC = importlib.util.spec_from_file_location("merge_queue_policy", SCRIPT)
policy = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(policy)

QUEUE = {"type": "merge_queue", "parameters": {"max_entries_to_merge": 5}}


def checks(*contexts, strict=False):
    return {
        "type": "required_status_checks",
        "ruleset_id": 1,
        "parameters": {
            "strict_required_status_checks_policy": strict,
            "required_status_checks": [{"context": context, "integration_id": 15368} for context in contexts],
        },
    }


NO_CLASSIC = {"protection": {"enabled": True, "required_status_checks": {"contexts": []}}}


class MergeQueuePolicyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.workflows = Path(self.temp.name) / "workflows"
        self.workflows.mkdir()
        self.write("ci.yml", "on:\n  pull_request:\n  merge_group:\n    types: [checks_requested]\n"
                   "jobs:\n  gate:\n    name: PR Gate\n  maintainer-gate:\n    runs-on: x\n")
        self.write("push.yaml", "on: [push, pull_request]\njobs:\n  only-pr:\n    name: PR only\n")
        self.write("list.yml", "on: [merge_group]\njobs:\n  scan:\n    name: ggshield\n")
        self.write("single.yml", "on: merge_group\njobs:\n  single:\n    name: Single\n")
        self.write("empty.yml", "")

    def write(self, name, text):
        (self.workflows / name).write_text(text, encoding="utf-8")

    def run_check(self, dev_rules, main_rules=(), dev_branch=None):
        return policy.check(list(dev_rules), list(main_rules), NO_CLASSIC if dev_branch is None else dev_branch,
                            self.workflows)

    def test_accepts_queue_with_merge_group_producers(self):
        self.assertEqual(self.run_check([QUEUE, checks("PR Gate", "maintainer-gate", "ggshield", "Single")]), [])

    def test_rejects_required_check_without_merge_group_producer(self):
        errors = self.run_check([QUEUE, checks("PR Gate", "PR only", "Unknown")])
        self.assertEqual(errors, [
            "Required dev check 'PR only' has no workflow job that runs on merge_group.",
            "Required dev check 'Unknown' has no workflow job that runs on merge_group.",
        ])

    def test_rejects_non_strict_checks_without_queue(self):
        errors = self.run_check([checks("PR Gate")])
        self.assertIn("dev has no merge queue rule.", errors)
        self.assertIn("Ruleset 1 has non-strict required checks on dev without a merge queue.", errors)

    def test_strict_checks_without_queue_report_only_missing_queue(self):
        self.assertEqual(self.run_check([checks("PR Gate", strict=True)]), ["dev has no merge queue rule."])

    def test_rejects_queue_on_main(self):
        errors = self.run_check([QUEUE, checks("PR Gate")], main_rules=[QUEUE])
        self.assertEqual(errors, ["main has a merge queue rule. Merges into main stay manual."])

    def test_rejects_classic_required_checks(self):
        branch = {"protection": {"required_status_checks": {"contexts": ["PR Gate"]}}}
        errors = self.run_check([QUEUE, checks("PR Gate")], dev_branch=branch)
        self.assertEqual(len(errors), 1)
        self.assertIn("classic branch protection: PR Gate", errors[0])

    def test_rejects_queue_without_required_checks(self):
        self.assertEqual(self.run_check([QUEUE], dev_branch={}),
                         ["dev has a merge queue but no required status checks."])

    def test_workflow_events_shapes(self):
        self.assertEqual(policy.workflow_events({"on": "push"}), {"push"})
        self.assertEqual(policy.workflow_events({True: ["push"]}), {"push"})
        self.assertEqual(policy.workflow_events({"on": {"merge_group": None}}), {"merge_group"})
        self.assertEqual(policy.workflow_events({}), set())

    def test_main_reports_result(self):
        files = {}
        for name, value in {"dev": [QUEUE, checks("PR Gate")], "main": [], "branch": NO_CLASSIC}.items():
            path = self.workflows.parent / f"{name}.json"
            path.write_text(json.dumps(value), encoding="utf-8")
            files[name] = str(path)
        argv = ["--dev-rules", files["dev"], "--main-rules", files["main"], "--dev-branch", files["branch"],
                "--workflows", str(self.workflows)]
        output = io.StringIO()
        with redirect_stdout(output):
            self.assertEqual(policy.main(argv), 0)
        self.assertIn("policy holds", output.getvalue())

        Path(files["dev"]).write_text(json.dumps([checks("PR only")]), encoding="utf-8")
        output = io.StringIO()
        with redirect_stdout(output):
            self.assertEqual(policy.main(argv), 1)
        self.assertIn("::error::dev has no merge queue rule.", output.getvalue())

    def test_repository_reads_live_rules(self):
        responses = {
            "repos/o/r/rules/branches/dev": [QUEUE, checks("ggshield")],
            "repos/o/r/rules/branches/main": [],
            "repos/o/r/branches/dev": NO_CLASSIC,
        }
        output = io.StringIO()
        with redirect_stdout(output):
            status = policy.main(["--repository", "o/r", "--workflows", str(self.workflows)],
                                 fetch=responses.__getitem__)
        self.assertEqual(status, 0)

    def test_file_inputs_need_all_three_files(self):
        with redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            policy.main(["--dev-rules", "dev.json"])

    def test_fetch_json_uses_gh_api(self):
        calls = []

        def run(command, **kwargs):
            calls.append((command, kwargs))
            return type("Result", (), {"stdout": "[1]"})()

        self.assertEqual(policy.fetch_json("repos/o/r", run=run), [1])
        self.assertEqual(calls[0][0], ["gh", "api", "repos/o/r"])
        self.assertTrue(calls[0][1]["check"])


if __name__ == "__main__":
    unittest.main()
