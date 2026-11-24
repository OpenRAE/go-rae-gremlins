"""Tests that maintainer integrations run only in the canonical repository (ADR 0010)."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

import yaml


WORKFLOWS = Path(__file__).resolve().parents[1] / "workflows"
# The ID of OpenRAE/go-rae-gremlins. A rename or a transfer keeps it; a fork has another ID.
GUARD = "github.repository_id == '1441387140'"

# Jobs and steps that need an OpenRAE credential, person, service, or branch setting.
MAINTAINER_ONLY = (
    ("ci.yml", "transport-release", None),
    ("ci.yml", "sonar-admission", None),
    ("ci.yml", "sonar", None),
    ("ci.yml", "policy", "Check the dev merge queue policy"),
    ("ggshield.yml", "ggshield", None),
    ("ground-control-fork-finalization.yml", "dispatch", None),
    ("ground-control-phase-e.yml", "finalize", None),
    ("maintainer-gate.yml", "maintainer-gate", None),
    ("merge-queue-removed.yml", "label", None),
    ("release-please.yml", "release-please", None),
    ("sync-main-to-dev.yml", "backmerge", None),
)

# Checks that every repository runs, also a fork without maintainer access.
NATIVE = (
    ("ci.yml", "architecture"),
    ("ci.yml", "lint"),
    ("ci.yml", "vuln"),
    ("ci.yml", "test"),
    ("ci.yml", "policy"),
    ("ci.yml", "docs"),
    ("ci.yml", "release-build"),
    ("ci.yml", "memory"),
    ("ci.yml", "transport-scope"),
    ("ci.yml", "gate"),
    ("gremlins.yml", "gremlins"),
    ("gremlins.yml", "regression-proof"),
    ("policy.yml", "policy"),
    ("pr-title.yml", "lint-title"),
)

SECRET = re.compile(r"secrets\.([A-Za-z_][A-Za-z0-9_]*)")


def load(name):
    return yaml.safe_load((WORKFLOWS / name).read_text(encoding="utf-8"))


def job(name, job_id):
    return load(name)["jobs"][job_id]


def step(name, job_id, step_name):
    matches = [item for item in job(name, job_id)["steps"] if item.get("name") == step_name]
    if len(matches) != 1:
        raise AssertionError(f"{name}: job {job_id} must have one step named {step_name!r}")
    return matches[0]


def expression(value):
    text = str(value or "").strip()
    if text.startswith("${{") and text.endswith("}}"):
        text = text[3:-2]
    return " ".join(text.split())


def has_top_level_or(text):
    depth = 0
    quoted = False
    for index, char in enumerate(text):
        if char == "'":
            quoted = not quoted
        elif not quoted and char == "(":
            depth += 1
        elif not quoted and char == ")":
            depth -= 1
        elif not quoted and depth == 0 and text.startswith("||", index):
            return True
    return False


def guarded(condition):
    """Return True when the repository condition applies to the full condition."""
    text = expression(condition)
    if text == GUARD:
        return True
    prefix = GUARD + " && "
    return text.startswith(prefix) and not has_top_level_or(text[len(prefix):])


def run_script(script, env):
    # GitHub runs `run` scripts with this bash command line.
    with tempfile.TemporaryDirectory() as temp:
        path = Path(temp) / "step.sh"
        output = Path(temp) / "output"
        output.touch()
        path.write_text(script, encoding="utf-8")
        result = subprocess.run(
            ["bash", "--noprofile", "--norc", "-eo", "pipefail", str(path)],
            env={"PATH": os.environ["PATH"], "GITHUB_OUTPUT": str(output), **env},
            capture_output=True,
            text=True,
            check=False,
        )
        return result, output.read_text(encoding="utf-8")


class ConditionParserTests(unittest.TestCase):
    def test_feature_guarded_accepts_only_a_full_condition(self):
        cases = (
            (GUARD, True),
            ("${{ " + GUARD + " }}", True),
            (GUARD + " && needs.scope.outputs.run == 'true'", True),
            (GUARD + " && (a || b)", True),
            (GUARD + " && a || b", False),
            (GUARD + " && ('x || y' == a || b)", True),
            ("a || " + GUARD, False),
            ("github.repository == 'OpenRAE/go-rae-gremlins'", False),
            ("", False),
        )
        for condition, want in cases:
            with self.subTest(condition=condition):
                self.assertEqual(guarded(condition), want)


class WorkflowApplicabilityTests(unittest.TestCase):
    def test_feature_maintainer_only_jobs_require_the_canonical_repository(self):
        for name, job_id, step_name in MAINTAINER_ONLY:
            with self.subTest(workflow=name, job=job_id, step=step_name):
                target = step(name, job_id, step_name) if step_name else job(name, job_id)
                self.assertTrue(guarded(target.get("if")), f"condition: {target.get('if')!r}")

    def test_feature_jobs_with_secrets_require_the_canonical_repository(self):
        for path in sorted(WORKFLOWS.glob("*.yml")):
            for job_id, body in load(path.name)["jobs"].items():
                secrets = set(SECRET.findall(json.dumps(body))) - {"GITHUB_TOKEN"}
                if not secrets:
                    continue
                with self.subTest(workflow=path.name, job=job_id, secrets=sorted(secrets)):
                    self.assertTrue(guarded(body.get("if")), f"condition: {body.get('if')!r}")

    def test_feature_only_listed_steps_have_a_repository_condition(self):
        listed = {(name, job_id, step_name) for name, job_id, step_name in MAINTAINER_ONLY if step_name}
        for path in sorted(WORKFLOWS.glob("*.yml")):
            for job_id, body in load(path.name)["jobs"].items():
                for item in body.get("steps", []):
                    if "repository_id" in expression(item.get("if")):
                        with self.subTest(workflow=path.name, job=job_id, step=item.get("name")):
                            self.assertIn((path.name, job_id, item.get("name")), listed)

    def test_feature_native_checks_run_in_every_repository(self):
        for name, job_id in NATIVE:
            with self.subTest(workflow=name, job=job_id):
                self.assertNotIn("repository_id", expression(job(name, job_id).get("if")))

    def test_feature_pr_gate_and_sonar_use_one_applicability_condition(self):
        gate_env = job("ci.yml", "gate")["steps"][0]["env"]
        self.assertEqual(expression(gate_env["SONAR_APPLIES"]), expression(job("ci.yml", "sonar-admission")["if"]))
        self.assertTrue(guarded(gate_env["SONAR_APPLIES"]))


class PrGateTests(unittest.TestCase):
    SUCCESS = {
        "ARCHITECTURE_RESULT": "success",
        "LINT_RESULT": "success",
        "VULN_RESULT": "success",
        "DOCS_RESULT": "success",
        "TEST_RESULT": "success",
        "RELEASE_BUILD_RESULT": "success",
        "POLICY_RESULT": "success",
        "MEMORY_RESULT": "success",
        "SONAR_APPLIES": "true",
        "SONAR_RESULT": "success",
        "TRANSPORT_SCOPE_RESULT": "success",
        "TRANSPORT_SELECTED": "false",
        "TRANSPORT_RESULT": "skipped",
    }

    def test_feature_pr_gate_accepts_only_applicable_results(self):
        script = job("ci.yml", "gate")["steps"][0]["run"]
        cases = (
            ("canonical event with all checks", {}, True),
            ("fork or foreign PR skips Sonar", {"SONAR_APPLIES": "false", "SONAR_RESULT": "skipped"}, True),
            ("applicable Sonar skipped", {"SONAR_RESULT": "skipped"}, False),
            ("applicable Sonar failed", {"SONAR_RESULT": "failure"}, False),
            ("applicable Sonar cancelled", {"SONAR_RESULT": "cancelled"}, False),
            ("inapplicable Sonar ran", {"SONAR_APPLIES": "false", "SONAR_RESULT": "success"}, False),
            ("selected transport passed", {"TRANSPORT_SELECTED": "true", "TRANSPORT_RESULT": "success"}, True),
            ("selected transport skipped", {"TRANSPORT_SELECTED": "true", "TRANSPORT_RESULT": "skipped"}, False),
            ("unselected transport ran", {"TRANSPORT_RESULT": "success"}, False),
            ("missing transport selection", {"TRANSPORT_SELECTED": ""}, False),
            ("invalid transport selection", {"TRANSPORT_SELECTED": "yes"}, False),
            ("transport scope failed", {"TRANSPORT_SCOPE_RESULT": "failure"}, False),
            ("native test skipped", {"TEST_RESULT": "skipped"}, False),
            ("native lint failed", {"LINT_RESULT": "failure"}, False),
        )
        for label, changes, want in cases:
            with self.subTest(case=label):
                result, _ = run_script(script, {**self.SUCCESS, **changes})
                self.assertEqual(result.returncode == 0, want, result.stderr)


class TransportScopeTests(unittest.TestCase):
    def setUp(self):
        selector = step("ci.yml", "transport-scope", "Select affected transport changes")
        self.script = selector["run"]
        self.env = selector["env"]

    def test_feature_scope_reads_the_repository_condition(self):
        self.assertEqual(expression(self.env["MAINTAINER_REPOSITORY"]), GUARD)

    def test_feature_scope_selects_nothing_outside_the_canonical_repository(self):
        for event in ("pull_request", "merge_group", "push"):
            with self.subTest(event=event):
                env = {"MAINTAINER_REPOSITORY": "false", "EVENT_NAME": event, "BASE": "", "NO_SECRETS": "false"}
                result, output = run_script(self.script, env)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(output, "run=false\n")

    def test_feature_canonical_scope_still_validates_its_base(self):
        env = {"MAINTAINER_REPOSITORY": "true", "EVENT_NAME": "pull_request", "BASE": "not-a-sha", "NO_SECRETS": "false"}
        result, output = run_script(self.script, env)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(output, "")


if __name__ == "__main__":
    unittest.main()
