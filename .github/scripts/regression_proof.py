"""Prove new TestRegression cases pass on head and fail on base production code.

With --per-change, prove each new case against the first-parent change that added it.
A promotion of dev to main uses this mode: a later dev change can alter an API that an
earlier regression test calls, so the test cannot compile against the main code.
"""

import argparse

import io
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile


def command(root, *args):
    return subprocess.check_output(args, cwd=root, timeout=120)


def go_events(root, *args):
    result = subprocess.run(
        ["go", "test", "-json", "-count=1", "-timeout=60s", *args],
        cwd=root, capture_output=True, text=True, timeout=120,
    )
    events = [json.loads(line) for line in result.stdout.splitlines() if line]
    return result.returncode, events, result.stdout + result.stderr


def discover(root, revision, packages=("./...",)):
    # CI runs both suites to discover dynamic t.Run names as well as functions.
    code, events, output = go_events(root, "-race", *packages)
    if code:
        raise ValueError(f"tests did not pass on {revision} during discovery:\n" + output[-6000:])
    module = module_path(root)
    return {
        ("." + event["Package"][len(module):], event["Test"])
        for event in events
        if event.get("Action") in {"run", "skip"}
        and "Package" in event and event.get("Test", "").startswith("Test")
    }


def module_path(root):
    return re.search(r"(?m)^module\s+(\S+)", (root / "go.mod").read_text())[1]


def is_test_file(path):
    return path.name.endswith("_test.go") or "testdata" in path.parts


def tracked_files(root):
    return command(root, "git", "ls-files", "-z").decode().split("\0")


def overlay_tests(root, base, tracked=None):
    """Copy the head tests and module files in tracked (default: git ls-files of root)."""
    head_module = module_path(root)
    base_module = module_path(base) if (base / "go.mod").exists() else head_module
    # Remove old tests/fixtures so renames and deletions cannot leave stale code.
    for path in sorted(base.rglob("*"), reverse=True):
        if is_test_file(path.relative_to(base)) and (path.is_file() or path.is_symlink()):
            path.unlink()
    for name in filter(None, tracked_files(root) if tracked is None else tracked):
        relative = Path(name)
        if not is_test_file(relative) and name not in {"go.mod", "go.sum"}:
            continue
        source = root / relative
        target = base / relative
        if source.is_symlink() or not source.resolve().is_relative_to(root):
            raise ValueError("unsafe test overlay: " + name)
        if not target.resolve().is_relative_to(base):
            raise ValueError("unsafe base overlay: " + name)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
        # Keep base production imports intact across a module-path migration.
        # Only the overlaid tests and module declaration need rebinding.
        if head_module != base_module and target.suffix == ".go":
            target.write_text(target.read_text().replace(head_module, base_module))
    if head_module != base_module:
        module_file = base / "go.mod"
        module_file.write_text(module_file.read_text().replace(
            "module " + head_module, "module " + base_module, 1))


def build_failed(events):
    return any(event.get("Action") == "build-fail" or event.get("FailedBuild")
               for event in events)


def observe(root, package, name):
    pattern = "/".join("^" + re.escape(part) + "$" for part in name.split("/"))
    code, events, output = go_events(root, "-race", "-run", pattern, package)
    actions = [event.get("Action") for event in events if event.get("Test") == name]
    skipped = any(event.get("Action") == "skip" and
                  (event.get("Test") == name or event.get("Test", "").startswith(name + "/"))
                  for event in events)
    passed = code == 0 and "run" in actions and "pass" in actions and not skipped
    failed = code != 0 and "run" in actions and "fail" in actions and not skipped
    # A timeout, panic, or package build failure is not an assertion proving the fix.
    failed = failed and "panic:" not in output and not build_failed(events)
    return passed, failed, build_failed(events), output


def declaring_files(root, package, name):
    """Return the head test files in package that declare the top-level test of name."""
    declaration = re.compile(r"(?m)^func\s+" + re.escape(name.split("/")[0]) + r"\s*\(")
    return {path.name for path in (root / package).glob("*_test.go")
            if declaration.search(path.read_text())}


def use_base_tests(repo, base, merge_base, package, keep):
    """Replace the overlaid test files of one package with base versions, except keep."""
    directory = base / package
    for path in directory.glob("*_test.go"):
        if path.name not in keep:
            path.unlink()
    listing = command(repo, "git", "ls-tree", "-z", "--name-only", merge_base,
                      "--", Path(package).as_posix() + "/")
    for name in filter(None, listing.decode().split("\0")):
        relative = Path(name)
        if not relative.name.endswith("_test.go") or relative.name in keep:
            continue
        target = base / relative
        if not target.resolve().is_relative_to(base):
            raise ValueError("unsafe base overlay: " + name)
        target.write_bytes(command(repo, "git", "show", f"{merge_base}:{name}"))


def extract(repo, revision, target):
    archive = command(repo, "git", "archive", "--format=tar", revision)
    with tarfile.open(fileobj=io.BytesIO(archive)) as snapshot:
        snapshot.extractall(target, filter="data")


def regression_leaves(added, tests):
    """Return the added TestRegression cases that have no subtests in tests."""
    # Prove leaves individually: a failing sibling cannot prove another case.
    return sorted((package, name) for package, name in added
                  if name.startswith("TestRegression") and not any(
                      pkg == package and child.startswith(name + "/")
                      for pkg, child in tests
                  ))


def prove_leaves(repo, head, base, merge_base, leaves, tracked=None):
    """Prove that each leaf passes on the head tree and fails by assertion on base."""
    overlay_tests(head, base, tracked)
    for package, name in leaves:
        passed, _, _, output = observe(head, package, name)
        if not passed:
            raise ValueError(f"{package}:{name} did not pass on head:\n{output[-6000:]}")
        passed, failed, build_failure, output = observe(base, package, name)
        note = ""
        if build_failure:
            # Other test files can use head-only APIs. Retry with only the files that
            # declare this regression; the package's other test files come from base.
            keep = declaring_files(head, package, name)
            use_base_tests(repo, base, merge_base, package, keep)
            passed, failed, _, output = observe(base, package, name)
            overlay_tests(head, base, tracked)
            note = "; other test files from base"
        if passed:
            raise ValueError(f"{package}:{name} passes on base; regression not proved")
        if not failed:
            raise ValueError(f"{package}:{name} not an assertion failure on base:\n{output[-6000:]}")
        print(f"{package}:{name} proved (head passes, base assertion fails{note})")


def test_package(name):
    """Return the package path of a test file or fixture, in the form discover uses."""
    parts = Path(name).parts
    directory = Path(*parts[:parts.index("testdata")]) if "testdata" in parts else Path(name).parent
    return "." if directory == Path(".") else "./" + directory.as_posix()


def prove_changes(repo, merge_base, pending):
    """Prove each pending leaf against the first-parent change that added it."""
    pending = set(pending)
    changes = command(repo, "git", "rev-list", "--first-parent", "--reverse",
                      f"{merge_base}..HEAD").decode().split()
    for change in changes:
        parent = command(repo, "git", "rev-parse", f"{change}^1").decode().strip()
        changed = command(repo, "git", "diff", "-z", "--name-only", parent, change).decode()
        touched = {test_package(name) for name in changed.split("\0")
                   if name and is_test_file(Path(name))}
        packages = sorted({package for package, _ in pending} & touched)
        if not packages:
            continue
        with tempfile.TemporaryDirectory(prefix="gremlins-change-") as temporary:
            head, base = Path(temporary, "head").resolve(), Path(temporary, "base").resolve()
            extract(repo, change, head)
            extract(repo, parent, base)
            head_packages = [package for package in packages if (head / package).is_dir()]
            base_packages = [package for package in packages if (base / package).is_dir()]
            tests = discover(head, change, head_packages) if head_packages else set()
            before = discover(base, parent, base_packages) if base_packages else set()
            leaves = [leaf for leaf in regression_leaves(tests - before, tests) if leaf in pending]
            if not leaves:
                continue
            print(f"Change {change[:12]}:")
            tracked = command(repo, "git", "ls-tree", "-r", "-z", "--name-only",
                              change).decode().split("\0")
            prove_leaves(repo, head, base, parent, leaves, tracked)
            pending -= set(leaves)
        if not pending:
            return
    raise ValueError("no first-parent change adds and proves: " + ", ".join(
        f"{package}:{name}" for package, name in sorted(pending)))


def prove(root, base_ref, per_change=False):
    head_tests = discover(root, "head")
    if not head_tests:
        print("No new TestRegression cases.")
        return
    merge_base = command(root, "git", "merge-base", "HEAD", base_ref).decode().strip()
    with tempfile.TemporaryDirectory(prefix="gremlins-proof-") as temporary:
        base = Path(temporary).resolve()
        extract(root, merge_base, base)
        added = head_tests - discover(base, "base")
        unclassified = sorted((package, name) for package, name in added
                              if not name.startswith(("TestRegression", "TestFeature")))
        if unclassified:
            raise ValueError("unclassified new test(s): " + ", ".join(
                f"{package}:{name}" for package, name in unclassified
            ) + "; use TestRegression for fixes or TestFeature for new behavior")
        new_tests = regression_leaves(added, head_tests)
        if not new_tests:
            print("No new TestRegression cases.")
            return
        if per_change:
            prove_changes(root, merge_base, new_tests)
            return
        prove_leaves(root, root, base, merge_base, new_tests)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--per-change", action="store_true",
                        help="prove each case against the first-parent change that added it")
    parser.add_argument("base_commit", metavar="BASE_COMMIT")
    arguments = parser.parse_args()
    root = Path(command(Path.cwd(), "git", "rev-parse", "--show-toplevel").decode().strip()).resolve()
    prove(root, arguments.base_commit, arguments.per_change)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError, tarfile.TarError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
