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

package report_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/execution"
	"github.com/OpenRAE/go-rae-gremlins/internal/log"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/report"
)

func mutantWith(status mutator.Status, col int, reason string) stubMutant {
	return stubMutant{
		status:           status,
		mutantType:       mutator.ConditionalsBoundary,
		position:         newPosition("a.go", col, 1),
		equivalentReason: reason,
	}
}

// mutantsOf returns k KILLED, l LIVED, u NOT COVERED, and e EQUIVALENT
// mutants.
func mutantsOf(k, l, u, e int) []mutator.Mutator {
	var ms []mutator.Mutator
	add := func(n int, s mutator.Status, reason string) {
		for range n {
			ms = append(ms, mutantWith(s, len(ms)+1, reason))
		}
	}
	add(k, mutator.Killed, "")
	add(l, mutator.Lived, "")
	add(u, mutator.NotCovered, "")
	add(e, mutator.Equivalent, "no observable change")

	return ms
}

// jsonReport is the part of the JSON report that the tests read. A nil
// EquivalentReason means that the report has no equivalent_reason field.
type jsonReport struct {
	MutantsEquivalent *int    `json:"mutants_equivalent"`
	MutantsTotal      int     `json:"mutants_total"`
	TestEfficacy      float64 `json:"test_efficacy"`
	MutationsCoverage float64 `json:"mutations_coverage"`
	Files             []struct {
		Mutations []struct {
			EquivalentReason *string `json:"equivalent_reason"`
			Column           int     `json:"column"`
		} `json:"mutations"`
	} `json:"files"`
}

// writtenReport runs report.Do on mutants with a JSON output file and
// returns the decoded file.
func writtenReport(t *testing.T, mutants []mutator.Mutator) jsonReport {
	t.Helper()
	output := filepath.Join(t.TempDir(), "report.json")
	configuration.Set(configuration.UnleashOutputKey, output)

	if err := report.Do(report.Results{Mutants: mutants, Elapsed: time.Second}); err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	//nolint:gosec // test code reading its own report file
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("os.ReadFile(report) error = %v, want nil", err)
	}
	var got jsonReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(report) error = %v, want nil", err)
	}

	return got
}

func TestFeatureReportWritesEquivalentReasons(t *testing.T) {
	isolateConfig(t)
	log.Init(&bytes.Buffer{}, &bytes.Buffer{})
	t.Cleanup(log.Reset)
	mutants := []mutator.Mutator{
		mutantWith(mutator.Equivalent, 1, "the bound is unreachable \"quoted\""),
		mutantWith(mutator.Skipped, 2, "outside the diff"),
		mutantWith(mutator.Killed, 3, ""),
	}

	got := writtenReport(t, mutants)

	if got.MutantsEquivalent == nil || *got.MutantsEquivalent != 1 {
		t.Errorf("mutants_equivalent = %v, want 1", got.MutantsEquivalent)
	}
	if got.MutantsTotal != 1 {
		t.Errorf("mutants_total = %d, want 1 (killed, lived, and not viable)", got.MutantsTotal)
	}
	if len(got.Files) != 1 {
		t.Fatalf("report has %d files, want 1", len(got.Files))
	}
	wantReasons := map[int]*string{1: ptr("the bound is unreachable \"quoted\""), 2: ptr("outside the diff"), 3: nil}
	gotReasons := map[int]*string{}
	for _, m := range got.Files[0].Mutations {
		gotReasons[m.Column] = m.EquivalentReason
	}
	if diff := cmp.Diff(wantReasons, gotReasons); diff != "" {
		t.Errorf("equivalent_reason by column mismatch (-want +got):\n%s", diff)
	}
}

func ptr(s string) *string {
	return &s
}

func TestFeatureThresholdsExcludeEquivalentMutants(t *testing.T) {
	testCases := []struct {
		name         string
		mutants      []mutator.Mutator
		efficacy     float64
		coverage     float64
		wantExitCode int
	}{
		{
			name:     "equivalent mutants are in neither ratio",
			mutants:  mutantsOf(1, 1, 2, 3),
			efficacy: 50,
			coverage: 50,
		},
		{
			name:         "efficacy below the threshold still fails",
			mutants:      mutantsOf(1, 1, 2, 3),
			efficacy:     51,
			wantExitCode: execution.NewExitErr(execution.EfficacyThreshold).ExitCode(),
		},
		{
			name:     "only equivalent mutants skip the assessment",
			mutants:  mutantsOf(0, 0, 0, 2),
			efficacy: 80,
			coverage: 90,
		},
		{
			name:     "only skipped and equivalent mutants skip the assessment",
			mutants:  append(mutantsOf(0, 0, 0, 1), mutantWith(mutator.Skipped, 9, "")),
			efficacy: 80,
			coverage: 90,
		},
		{
			name:         "an equivalent mutant does not hide an uncovered mutant",
			mutants:      mutantsOf(0, 0, 1, 1),
			coverage:     90,
			wantExitCode: execution.NewExitErr(execution.MutantCoverageThreshold).ExitCode(),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			log.Init(&bytes.Buffer{}, &bytes.Buffer{})
			t.Cleanup(log.Reset)
			configuration.Set(configuration.UnleashThresholdEfficacyKey, tc.efficacy)
			configuration.Set(configuration.UnleashThresholdMCoverageKey, tc.coverage)

			err := report.Do(report.Results{Mutants: tc.mutants, Elapsed: time.Second})

			var exitErr *execution.ExitError
			switch {
			case tc.wantExitCode == 0 && err != nil:
				t.Errorf("Do() error = %v, want nil", err)
			case tc.wantExitCode != 0 && !errors.As(err, &exitErr):
				t.Errorf("Do() error = %v, want exit code %d", err, tc.wantExitCode)
			case tc.wantExitCode != 0 && exitErr.ExitCode() != tc.wantExitCode:
				t.Errorf("Do() exit code = %d, want %d", exitErr.ExitCode(), tc.wantExitCode)
			}
		})
	}
}

func TestFeatureReportRatiosExcludeEquivalentMutants(t *testing.T) {
	isolateConfig(t)
	log.Init(&bytes.Buffer{}, &bytes.Buffer{})
	t.Cleanup(log.Reset)

	got := writtenReport(t, mutantsOf(3, 1, 4, 5))

	if got.TestEfficacy != 75 {
		t.Errorf("test_efficacy = %v, want 75 (3 / (3 + 1))", got.TestEfficacy)
	}
	if got.MutationsCoverage != 50 {
		t.Errorf("mutations_coverage = %v, want 50 ((3 + 1) / (3 + 1 + 4))", got.MutationsCoverage)
	}
}

func TestFeatureReportPrintsEquivalentCount(t *testing.T) {
	testCases := []struct {
		name   string
		dryRun bool
		want   string
	}{
		{
			name: "full run",
			want: "\n" +
				"Mutation testing completed in 1 second\n" +
				"Killed: 1, Lived: 0, Not covered: 0\n" +
				"Timed out: 0, Memory errors: 0, Not viable: 0, Skipped: 0\n" +
				"Equivalent: 2\n" +
				"Test efficacy: 100.00%\n" +
				"Mutator coverage: 100.00%\n",
		},
		{
			name:   "dry run",
			dryRun: true,
			want: "\n" +
				"Dry run completed in 1 second\n" +
				"Runnable: 0, Not covered: 1\n" +
				"Equivalent: 2\n" +
				"Mutator coverage: 0.00%\n",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			configuration.Set(configuration.UnleashDryRunKey, tc.dryRun)
			out := &bytes.Buffer{}
			log.Init(out, &bytes.Buffer{})
			t.Cleanup(log.Reset)
			mutants := mutantsOf(1, 0, 0, 2)
			if tc.dryRun {
				mutants = mutantsOf(0, 0, 1, 2)
			}

			if err := report.Do(report.Results{Mutants: mutants, Elapsed: time.Second}); err != nil {
				t.Fatalf("Do() error = %v, want nil", err)
			}

			if diff := cmp.Diff(tc.want, out.String()); diff != "" {
				t.Errorf("Do() output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFeatureMutantLogsEquivalentStatus(t *testing.T) {
	out := &bytes.Buffer{}
	log.Init(out, &bytes.Buffer{})
	t.Cleanup(log.Reset)

	report.Mutant(stubMutant{status: mutator.Equivalent, mutantType: mutator.ConditionalsBoundary, position: fakePosition})

	want := "  EQUIVALENT CONDITIONALS_BOUNDARY at aFolder/aFile.go:12:3\n"
	if diff := cmp.Diff(want, out.String()); diff != "" {
		t.Errorf("Mutant() output mismatch (-want +got):\n%s", diff)
	}
}

func TestFeatureParseFilterSelectsEquivalent(t *testing.T) {
	got, err := report.ParseFilter("e")
	if err != nil {
		t.Fatalf("ParseFilter(%q) = %v, want nil", "e", err)
	}
	want := report.Filter{mutator.Equivalent: struct{}{}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ParseFilter(%q) mismatch (-want +got):\n%s", "e", diff)
	}
}
