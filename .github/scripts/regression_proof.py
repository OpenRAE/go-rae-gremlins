"""Prove new TestRegression cases pass on head and fail on base production code."""

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


def discover(root, revision):
    # CI runs both suites to discover dynamic t.Run names as well as functions.
    code, events, output = go_events(root, "-race", "./...")
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


def overlay_tests(root, base):
    head_module = module_path(root)
    base_module = module_path(base) if (base / "go.mod").exists() else head_module
    # Remove old tests/fixtures so renames and deletions cannot leave stale code.
    for path in sorted(base.rglob("*"), reverse=True):
        if is_test_file(path.relative_to(base)) and (path.is_file() or path.is_symlink()):
            path.unlink()
    tracked = command(root, "git", "ls-files", "-z").decode().split("\0")
    for name in filter(None, tracked):
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


def use_base_tests(root, base, merge_base, package, keep):
    """Replace the overlaid test files of one package with base versions, except keep."""
    directory = base / package
    for path in directory.glob("*_test.go"):
        if path.name not in keep:
            path.unlink()
    listing = command(root, "git", "ls-tree", "-z", "--name-only", merge_base,
                      "--", Path(package).as_posix() + "/")
    for name in filter(None, listing.decode().split("\0")):
        relative = Path(name)
        if not relative.name.endswith("_test.go") or relative.name in keep:
            continue
        target = base / relative
        if not target.resolve().is_relative_to(base):
            raise ValueError("unsafe base overlay: " + name)
        target.write_bytes(command(root, "git", "show", f"{merge_base}:{name}"))


def prove(root, base_ref):
    head_tests = discover(root, "head")
    if not head_tests:
        print("No new TestRegression cases.")
        return
    merge_base = command(root, "git", "merge-base", "HEAD", base_ref).decode().strip()
    with tempfile.TemporaryDirectory(prefix="gremlins-proof-") as temporary:
        base = Path(temporary).resolve()
        archive = command(root, "git", "archive", "--format=tar", merge_base)
        with tarfile.open(fileobj=io.BytesIO(archive)) as snapshot:
            snapshot.extractall(base, filter="data")
        added = head_tests - discover(base, "base")
        unclassified = sorted((package, name) for package, name in added
                              if not name.startswith(("TestRegression", "TestFeature")))
        if unclassified:
            raise ValueError("unclassified new test(s): " + ", ".join(
                f"{package}:{name}" for package, name in unclassified
            ) + "; use TestRegression for fixes or TestFeature for new behavior")
        # Prove leaves individually: a failing sibling cannot prove another case.
        new_tests = sorted((package, name) for package, name in added
                           if name.startswith("TestRegression") and not any(
                               pkg == package and child.startswith(name + "/")
                               for pkg, child in head_tests
                           ))
        if not new_tests:
            print("No new TestRegression cases.")
            return
        overlay_tests(root, base)
        for package, name in new_tests:
            passed, _, _, output = observe(root, package, name)
            if not passed:
                raise ValueError(f"{package}:{name} did not pass on head:\n{output[-6000:]}")
            passed, failed, build_failure, output = observe(base, package, name)
            note = ""
            if build_failure:
                # Other test files can use head-only APIs. Retry with only the files that
                # declare this regression; the package's other test files come from base.
                keep = declaring_files(root, package, name)
                use_base_tests(root, base, merge_base, package, keep)
                passed, failed, _, output = observe(base, package, name)
                overlay_tests(root, base)
                note = "; other test files from base"
            if passed:
                raise ValueError(f"{package}:{name} passes on base; regression not proved")
            if not failed:
                raise ValueError(f"{package}:{name} not an assertion failure on base:\n{output[-6000:]}")
            print(f"{package}:{name} proved (head passes, base assertion fails{note})")


def main():
    if len(sys.argv) != 2:
        raise ValueError("usage: regression_proof.py BASE_COMMIT")
    root = Path(command(Path.cwd(), "git", "rev-parse", "--show-toplevel").decode().strip()).resolve()
    prove(root, sys.argv[1])


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError, tarfile.TarError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
