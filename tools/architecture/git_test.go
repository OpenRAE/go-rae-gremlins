/*
 * Copyright 2026 The Gremlins Authors
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func gitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Test"},
		{"config", "user.email", "test@example.invalid"},
	} {
		if _, err := gitRead(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, root, "README.md", "base\n")
	commitFixture(t, root)

	return root
}
func commitFixture(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"add", "--all"}, {"-c", "core.hooksPath=/dev/null", "commit", "-qm", "fixture"}} {
		if _, err := gitRead(root, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFeatureGitBaselineHistory(t *testing.T) {
	root := gitFixture(t)
	b, revision, err := targetBaseline(root, "HEAD")
	if err != nil || b.Version != 1 || len(b.Findings) != 0 || revision == "" {
		t.Fatalf("targetBaseline(HEAD) on first adoption = %+v, %q, %v; want version 1, no findings, a revision, nil",
			b, revision, err)
	}
	writeFile(t, root, baselineFile, `{"version":1,"findings":[{"rule":"DEP","file":"core/a.go","subject":"app"}]}`)
	commitFixture(t, root)
	b, _, err = targetBaseline(root, "HEAD")
	if err != nil || len(b.Findings) != 1 {
		t.Fatalf("targetBaseline(HEAD) = %+v, %v; want one finding, nil", b, err)
	}
	for _, ref := range []string{"missing-history", "--help", "HEAD:README.md"} {
		if _, _, err := targetBaseline(root, ref); err == nil {
			t.Errorf("accepted ref %q", ref)
		}
	}
}

func TestFeatureChangedDesignGate(t *testing.T) {
	root := gitFixture(t)
	c := testModel(t)
	writeFile(t, root, "core/a.go", "package core")
	if err := checkChangedDesign(root, "HEAD", c); err == nil {
		t.Fatal("missing design accepted")
	}
	d := design{
		Version:         1,
		Issue:           76,
		Summary:         changeSummary,
		TrustChanges:    localTrust(),
		TrustBoundaries: localBoundary,
		Changes:         []change{{Package: appPackage, Imports: []string{corePackage}}},
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, proposalFile, string(data))
	if err := checkChangedDesign(root, "HEAD", c); err == nil {
		t.Fatal("uncovered package accepted")
	}
	d.Changes = []change{{Package: corePackage, Imports: nil}}
	data, err = json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, proposalFile, string(data))
	if err := checkChangedDesign(root, "HEAD", c); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureMalformedTargetBaseline(t *testing.T) {
	root := gitFixture(t)
	writeFile(t, root, baselineFile, `{"version":1,"version":2,"findings":[]}`)
	commitFixture(t, root)
	if _, _, err := targetBaseline(root, "HEAD"); err == nil {
		t.Fatal("accepted duplicate target policy")
	}
}

func TestFeatureGitDiagnosticDoesNotLeakSubprocessOutput(t *testing.T) {
	root := gitFixture(t)
	_, err := gitRead(root, "show", "nonexistent:private-file")
	if err == nil || strings.Contains(err.Error(), "private-file") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func writeDesign(t *testing.T, root string, d design) {
	t.Helper()
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, proposalFile, string(data))
}

func TestFeatureContractAndPolicyChangesNeedDesign(t *testing.T) {
	for _, tc := range []struct {
		name, path          string
		contracts, policies []string
		contractChanges     map[string]string
	}{
		{contractKind, "schema.json", []string{cliContract}, nil, map[string]string{cliContract: compatibleChange}},
		{"policy", modelFile, nil, []string{"model"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gitFixture(t)
			c := testModel(t)
			c.Contracts[cliContract] = "schema.json"
			c.Policies = map[string]string{"model": modelFile}
			writeFile(t, root, tc.path, "changed")
			if err := checkChangedDesign(root, "HEAD", c); err == nil {
				t.Fatal("modeled change passed without a design")
			}
			d := design{
				Version: 1, Issue: 76, Summary: changeSummary,
				TrustChanges: localTrust(), TrustBoundaries: "reviewed",
			}
			writeDesign(t, root, d)
			if err := checkChangedDesign(root, "HEAD", c); err == nil {
				t.Fatal("undeclared modeled change passed")
			}
			d.Contracts = tc.contracts
			d.Policies = tc.policies
			d.ContractChanges = tc.contractChanges
			writeDesign(t, root, d)
			if err := checkChangedDesign(root, "HEAD", c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFeaturePreCodeDesignStep(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash design step runs in the Linux architecture CI job")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input, rule, subject string
		bad                        bool
	}{
		{"valid", "tools/architecture/testdata/valid-design.json", "", "", false},
		{"forbidden", "tools/architecture/testdata/invalid-design.json", "ARCH002", "cmd", true},
		{"breaking contract", "tools/architecture/testdata/invalid-contract.json", "ARCH003", "cli", true},
		{"prohibited transfer", "tools/architecture/testdata/invalid-trust.json", "ARCH004", "development_checks", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "scripts/check-design.sh", tc.input) //nolint:gosec // Literal test fixture paths.
			cmd.Dir = root
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.bad {
				t.Fatalf("check-design.sh %s error = %v, want error %t; output:\n%s", tc.input, err, tc.bad, output)
			}
			if tc.bad && !strings.Contains(string(output), tc.rule) {
				t.Fatalf("design step did not evaluate the import rule: %s", output)
			}
			if tc.bad && !strings.Contains(string(output), `subject="`+tc.subject+`"`) {
				t.Fatalf("design step did not reject subject %q: %s", tc.subject, output)
			}
		})
	}
}

func TestFeatureDesignOnlyChangeEvaluated(t *testing.T) {
	root := gitFixture(t)
	writeFile(t, root, proposalFile, `{
  "version": 1,
  "issue": 76,
  "summary": "proposal",
  "trust_boundaries": "local",
  "trust_changes": {
    "checks": {
      "destinations": [
        "new-service"
      ]
    }
  }
}`)
	if err := checkChangedDesign(root, "HEAD", testModel(t)); err == nil {
		t.Fatal("incompatible design-only change was not checked")
	}
}

func TestFeaturePreCodeDesignChange(t *testing.T) {
	root := gitFixture(t)
	data, err := json.Marshal(testModel(t).model)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, modelFile, string(data))
	writeDesign(t, root, design{
		Version:         1,
		Issue:           76,
		Summary:         changeSummary,
		TrustChanges:    localTrust(),
		TrustBoundaries: localBoundary,
		Changes:         []change{{Package: appPackage, Imports: []string{corePackage}}},
	})
	t.Chdir(root)
	if err := run("HEAD", "", true); err != nil {
		t.Fatalf("valid changed design rejected: %v", err)
	}
	writeFile(t, root, proposalFile, `{"version":2}`)
	if err := run("HEAD", "", true); err == nil {
		t.Fatal("invalid changed design accepted")
	}
}

func TestFeatureCLIContractSourcesRequireCompatibilityReview(t *testing.T) {
	checkout, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var m model
	if err := loadJSON(checkout, modelFile, &m); err != nil {
		t.Fatal(err)
	}
	c, err := compile(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ file, contract string }{
		{"cmd/gremlins.go", "cli_persistent"}, {"cmd/internal/flags/flags.go", "cli_binding"},
	} {
		t.Run(tc.contract, func(t *testing.T) {
			root := gitFixture(t)
			writeFile(t, root, tc.file, "package cmd")
			d := design{
				Version:         1,
				Issue:           76,
				Summary:         changeSummary,
				TrustBoundaries: localBoundary,
				TrustChanges:    c.TrustBoundaries,
				Changes:         []change{{Package: packageOf(tc.file), Imports: nil}},
			}
			data, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, root, proposalFile, string(data))
			if err := checkChangedDesign(root, "HEAD", c); err == nil {
				t.Fatal("CLI source changed without compatibility review")
			}
			d.Contracts = []string{tc.contract}
			d.ContractChanges = map[string]string{tc.contract: compatibleChange}
			data, err = json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, root, proposalFile, string(data))
			if err := checkChangedDesign(root, "HEAD", c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// writeModel writes a model that adds one trust boundary for each name, and returns the compiled model.
func writeModel(t *testing.T, root string, boundaries ...string) *compiledModel {
	t.Helper()
	m := testModel(t).model
	for _, name := range boundaries {
		m.TrustBoundaries[name] = trustBoundary{Destinations: []string{localBoundary}}
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, modelFile, string(data))
	c, err := compile(m)
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func writeTrustDesign(t *testing.T, root, file string, boundaries ...string) {
	t.Helper()
	trust := localTrust()
	for _, name := range boundaries {
		trust[name] = trustBoundary{Destinations: []string{localBoundary}}
	}
	data, err := json.Marshal(design{
		Version: 1, Issue: 76, Summary: changeSummary, TrustChanges: trust, TrustBoundaries: localBoundary,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, file, string(data))
}

func TestFeatureDesignUsesItsCommitModel(t *testing.T) {
	const later = ".architecture/designs/77.json"
	root := gitFixture(t)
	base, err := resolveRevision(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	writeModel(t, root)
	writeTrustDesign(t, root, proposalFile)
	commitFixture(t, root)
	c := writeModel(t, root, "release")
	writeTrustDesign(t, root, later, "release")
	commitFixture(t, root)
	paths := []string{proposalFile, later}
	if _, err := checkDesigns(root, base, c, paths); err != nil {
		t.Fatalf("earlier design rejected by a later model change: %v", err)
	}

	c = writeModel(t, root, "release", "extra")
	commitFixture(t, root)
	_, err = checkDesigns(root, base, c, paths)
	if err == nil || !strings.Contains(err.Error(), "no changed design passes the current model") {
		t.Fatalf("checkDesigns() after an uncovered model change = %v; want current-model error", err)
	}

	writeTrustDesign(t, root, later, "release", "extra")
	if _, err := checkDesigns(root, base, c, paths); err != nil {
		t.Fatalf("uncommitted design for the current model rejected: %v", err)
	}
	writeTrustDesign(t, root, proposalFile, "unknown-boundary")
	_, err = checkDesigns(root, base, c, paths)
	if err == nil || !strings.Contains(err.Error(), proposalFile) {
		t.Fatalf("checkDesigns() with an invalid uncommitted design = %v; want error for %s", err, proposalFile)
	}
}

func TestFeatureDesignCommitNeedsModel(t *testing.T) {
	root := gitFixture(t)
	base, err := resolveRevision(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	writeTrustDesign(t, root, proposalFile)
	commitFixture(t, root)
	_, err = checkDesigns(root, base, testModel(t), []string{proposalFile})
	if err == nil || !strings.Contains(err.Error(), "missing model") {
		t.Fatalf("checkDesigns() without a committed model = %v; want missing-model error", err)
	}
}
