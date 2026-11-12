"""Check that the dev merge queue can merge only pull requests that pass every required check.

The script reads the branch rules that GitHub applies to dev and to main, the classic
branch protection summary of dev, and the workflow files. It fails when:

- dev has no merge queue, or main has a merge queue;
- dev has required status checks in classic branch protection, where a token without
  admin access cannot read the strict setting;
- dev has non-strict required status checks and no merge queue;
- a required dev check has no GitHub Actions job that runs on the merge_group event.
"""

import argparse
import json
from pathlib import Path
import subprocess
import sys

import yaml


def load_json(path):
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def fetch_json(endpoint, run=subprocess.run):
    """Read one GitHub API endpoint with the gh CLI."""
    result = run(["gh", "api", endpoint], check=True, capture_output=True, text=True)
    return json.loads(result.stdout)


def workflow_events(document):
    """Return the event names of a parsed workflow. YAML 1.1 reads the key `on` as True."""
    events = document.get("on", document.get(True))
    if isinstance(events, str):
        return {events}
    if isinstance(events, list):
        return set(events)
    if isinstance(events, dict):
        return set(events)
    return set()


def merge_group_producers(workflow_dir):
    """Return the check names of jobs in workflows that run on merge_group."""
    producers = set()
    for path in sorted(Path(workflow_dir).glob("*.y*ml")):
        with open(path, encoding="utf-8") as handle:
            document = yaml.safe_load(handle) or {}
        if "merge_group" not in workflow_events(document):
            continue
        for job_id, job in (document.get("jobs") or {}).items():
            producers.add(str((job or {}).get("name", job_id)))
    return producers


def rules_of_type(rules, kind):
    return [rule for rule in rules if rule.get("type") == kind]


def check(dev_rules, main_rules, dev_branch, workflow_dir):
    """Return the list of policy errors."""
    errors = []
    queue = bool(rules_of_type(dev_rules, "merge_queue"))
    if not queue:
        errors.append("dev has no merge queue rule.")
    if rules_of_type(main_rules, "merge_queue"):
        errors.append("main has a merge queue rule. Merges into main stay manual.")

    classic = ((dev_branch.get("protection") or {}).get("required_status_checks") or {}).get("contexts") or []
    if classic:
        errors.append(
            "dev has required checks in classic branch protection: "
            f"{', '.join(sorted(classic))}. Move them to a ruleset so that this check can read the strict setting."
        )

    required = set()
    for rule in rules_of_type(dev_rules, "required_status_checks"):
        parameters = rule.get("parameters") or {}
        if not parameters.get("strict_required_status_checks_policy") and not queue:
            errors.append(f"Ruleset {rule.get('ruleset_id')} has non-strict required checks on dev without a merge queue.")
        for item in parameters.get("required_status_checks") or []:
            required.add(item["context"])
    if queue and not required:
        errors.append("dev has a merge queue but no required status checks.")

    producers = merge_group_producers(workflow_dir)
    for context in sorted(required - producers):
        errors.append(f"Required dev check '{context}' has no workflow job that runs on merge_group.")
    return errors


def main(argv=None, fetch=fetch_json):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--repository", help="owner/name; read the live rules with gh api")
    source.add_argument("--dev-rules", help="JSON from GET /repos/{repo}/rules/branches/dev")
    parser.add_argument("--main-rules", help="JSON from GET /repos/{repo}/rules/branches/main")
    parser.add_argument("--dev-branch", help="JSON from GET /repos/{repo}/branches/dev")
    parser.add_argument("--workflows", default=".github/workflows")
    args = parser.parse_args(argv)
    if args.repository:
        base = f"repos/{args.repository}"
        inputs = [fetch(f"{base}/rules/branches/dev"), fetch(f"{base}/rules/branches/main"),
                  fetch(f"{base}/branches/dev")]
    elif args.main_rules and args.dev_branch:
        inputs = [load_json(args.dev_rules), load_json(args.main_rules), load_json(args.dev_branch)]
    else:
        parser.error("--dev-rules needs --main-rules and --dev-branch")
    errors = check(*inputs, args.workflows)
    for error in errors:
        print(f"::error::{error}")
    if not errors:
        print("The dev merge queue policy holds.")
    return 1 if errors else 0

if __name__ == "__main__":
    sys.exit(main())
