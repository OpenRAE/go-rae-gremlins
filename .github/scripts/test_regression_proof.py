"""Integration tests for the PR regression proof check using tiny Go repositories."""

import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("regression_proof.py")


class RegressionProofTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.write("go.mod", "module example.test/proof\n\ngo 1.25.0\n")
        self.write("value.go", "package proof\nfunc Value() int { return 1 }\n")
        self.git("init", "-q")
        self.git("add", ".")
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.test",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "base")
        self.base = self.git("rev-parse", "HEAD").strip()

    def write(self, name, contents):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(contents)

    def git(self, *args):
        return subprocess.check_output(["git", *args], cwd=self.root, text=True)

    def regression(self, body):
        self.write("value_test.go", 'package proof\nimport "testing"\n'
                   'func TestRegressionValue(t *testing.T) { ' + body + ' }\n')
        self.git("add", ".")

    def proof(self):
        return subprocess.run([sys.executable, str(SCRIPT), self.base],
                              cwd=self.root, text=True, capture_output=True,
                              env={**os.environ, "GOWORK": "off"}, timeout=90)

    def test_rejects_test_passing_on_base(self):
        self.regression('if Value() != 1 { t.Fatal("wrong value") }')
        result = self.proof()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("passes on base", result.stderr)

    def test_rejects_unclassified_new_test(self):
        self.write("value_test.go", 'package proof\nimport "testing"\n'
                   'func TestValue(t *testing.T) {}\n')
        self.git("add", ".")
        result = self.proof()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("unclassified new test", result.stderr)

    def test_allows_explicit_feature_test(self):
        self.write("value_test.go", 'package proof\nimport "testing"\n'
                   'func TestFeatureNewValue(t *testing.T) { '
                   'if NewValue() != 2 { t.Fatal("wrong value") } }\n')
        self.write("value.go", "package proof\nfunc NewValue() int { return 2 }\n")
        self.git("add", ".")
        result = self.proof()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("No new TestRegression", result.stdout)

    def test_accepts_assertion_failure_on_base(self):
        self.regression('if Value() != 2 { t.Fatal("wrong value") }')
        self.write("value.go", "package proof\nfunc Value() int { return 2 }\n")
        result = self.proof()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("proved", result.stdout)

    def test_rejects_skipped_test(self):
        self.regression('t.Skip("not a proof")')
        result = self.proof()
        self.assertEqual(result.returncode, 1)
        self.assertIn("did not pass on head", result.stderr)

    def test_rejects_build_failure_on_base(self):
        self.regression('if NewValue() != 2 { t.Fatal("wrong value") }')
        self.write("value.go", "package proof\nfunc NewValue() int { return 2 }\n")
        result = self.proof()
        self.assertEqual(result.returncode, 1)
        self.assertIn("not an assertion failure on base", result.stderr)

    def test_no_regressions(self):
        result = self.proof()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("No new TestRegression", result.stdout)

    def test_checks_each_test_separately(self):
        self.regression('if Value() != 2 { t.Fatal("wrong value") }')
        with (self.root / "value_test.go").open("a") as target:
            target.write('func TestRegressionFalseProof(t *testing.T) {}\n')
        self.write("value.go", "package proof\nfunc Value() int { return 2 }\n")
        result = self.proof()
        self.assertEqual(result.returncode, 1)
        self.assertIn("TestRegressionFalseProof passes on base", result.stderr)

    def test_checks_each_new_subtest_separately(self):
        self.regression('t.Run("existing", func(t *testing.T) {})')
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.test",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "existing regression")
        self.base = self.git("rev-parse", "HEAD").strip()
        self.regression('t.Run("existing", func(t *testing.T) {}); '
                        't.Run("real", func(t *testing.T) { '
                        'if Value() != 2 { t.Fatal("wrong value") } }); '
                        't.Run("false", func(t *testing.T) {})')
        self.write("value.go", "package proof\nfunc Value() int { return 2 }\n")
        result = self.proof()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("TestRegressionValue/false passes on base", result.stderr)

    def test_rejects_unclassified_subtest_in_existing_test(self):
        self.write("value_test.go", 'package proof\nimport "testing"\n'
                   'func TestValue(t *testing.T) { t.Run("existing", func(t *testing.T) {}) }\n')
        self.git("add", ".")
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.test",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "existing test")
        self.base = self.git("rev-parse", "HEAD").strip()
        self.write("value_test.go", 'package proof\nimport "testing"\n'
                   'func TestValue(t *testing.T) { t.Run("existing", func(t *testing.T) {}); '
                   't.Run("new", func(t *testing.T) {}) }\n')
        result = self.proof()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("unclassified new test", result.stderr)

    def test_accepts_new_regression_subtest(self):
        self.regression('t.Run("existing", func(t *testing.T) {})')
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.test",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "existing regression")
        self.base = self.git("rev-parse", "HEAD").strip()
        self.regression('t.Run("existing", func(t *testing.T) {}); '
                        't.Run("new", func(t *testing.T) { '
                        'if Value() != 2 { t.Fatal("wrong value") } })')
        self.write("value.go", "package proof\nfunc Value() int { return 2 }\n")
        result = self.proof()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("TestRegressionValue/new proved", result.stdout)

    def test_uses_test_fixtures(self):
        self.regression('if Value() != 2 { t.Fatal("wrong value") }')
        self.write("testdata/expected", "2")
        self.write("fixture_test.go", 'package proof\nimport "os"\n'
                   'func init() { if _, err := os.ReadFile("testdata/expected"); '
                   'err != nil { panic(err) } }\n')
        self.git("add", ".")
        self.write("value.go", "package proof\nfunc Value() int { return 2 }\n")
        result = self.proof()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_existing_regression_is_not_new_proof(self):
        self.regression('if Value() != 1 { t.Fatal("wrong value") }')
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.test",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "existing test")
        self.base = self.git("rev-parse", "HEAD").strip()
        result = self.proof()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("No new TestRegression", result.stdout)

    def test_rejects_failure_on_head(self):
        self.regression('t.Fatal("broken test")')
        result = self.proof()
        self.assertEqual(result.returncode, 1)
        self.assertIn("did not pass on head", result.stderr)

    def helper(self):
        spec = importlib.util.spec_from_file_location("regression_proof", SCRIPT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_rejects_symlink_overlay(self):
        self.write("target", "private fixture")
        (self.root / "fixture_test.go").symlink_to("target")
        self.git("add", ".")
        with tempfile.TemporaryDirectory() as temporary:
            with self.assertRaisesRegex(ValueError, "unsafe test overlay"):
                self.helper().overlay_tests(self.root, Path(temporary).resolve())

    def test_rejects_base_directory_escape(self):
        self.regression('t.Fatal("regression")')
        self.write("nested/testdata/fixture", "safe")
        self.git("add", ".")
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary).resolve()
            (base / "nested").symlink_to(self.root, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "unsafe base overlay"):
                self.helper().overlay_tests(self.root, base)


if __name__ == "__main__":
    unittest.main()
