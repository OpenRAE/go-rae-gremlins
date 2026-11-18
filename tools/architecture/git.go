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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// gitRead uses fixed commands and never runs hooks, shell code, or generators.
func gitRead(root string, args ...string) ([]byte, error) {
	fixed := append([]string{"-C", root, "--no-pager", "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.Command("git", fixed...) //nolint:gosec // Fixed Git commands; refs are resolved with --end-of-options.
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git history read failed; fix: fetch the pinned comparison commit and retry")
	}

	return data, nil
}

func targetBaseline(root, ref string) (baseline, string, error) {
	empty := baseline{Version: 1, Findings: []finding{}}
	if ref == "" {
		return empty, "", nil
	}
	revision, err := resolveRevision(root, ref)
	if err != nil {
		return baseline{}, "", err
	}
	data, err := commitFile(root, revision, baselineFile)
	if err != nil {
		return baseline{}, "", err
	}
	if data == nil {
		return empty, revision, nil
	}
	var b baseline
	if err := decode(data, &b); err != nil {
		return baseline{}, "", err
	}

	return b, revision, nil
}

// commitFile reads a regular file from a commit, and returns nil data when the commit has no such file.
func commitFile(root, revision, file string) ([]byte, error) {
	listing, err := gitRead(root, "ls-tree", revision, "--", file)
	if err != nil || len(listing) == 0 {
		return nil, err
	}
	// A tree symlink or submodule must never become policy data.
	if !bytes.HasPrefix(listing, []byte("100644 blob ")) && !bytes.HasPrefix(listing, []byte("100755 blob ")) {
		return nil, fmt.Errorf("invalid %s mode at the comparison commit; fix: restore a regular file", file)
	}

	return gitRead(root, "show", revision+":"+file)
}

func changedPaths(root, revision string) ([]string, error) {
	data, err := gitRead(root, "diff", "--no-ext-diff", "--no-textconv", "--name-only", "-z", revision, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := gitRead(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	data = append(data, untracked...)
	var paths []string
	for _, p := range strings.Split(string(data), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}

	return paths, nil
}

func changedPackage(file string) bool {
	for _, part := range strings.Split(file, "/") {
		if excludedDir(part) {
			return false
		}
	}

	return strings.HasSuffix(file, ".go")
}

// resolveRevision turns a comparison ref into a commit, and keeps an empty base as first adoption.
func resolveRevision(root, base string) (string, error) {
	if base == "" {
		return "", nil
	}
	raw, err := gitRead(root, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(raw)), nil
}

// isDesignFile reports whether a changed path is a per-change design file in the design directory.
func isDesignFile(p string) bool {
	return path.Dir(p) == designDir && strings.HasSuffix(p, ".json")
}

// regularFile reports whether a checkout-relative path is a regular file, so a deleted design is skipped.
func regularFile(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))

	return err == nil && info.Mode().IsRegular()
}

// designsFromPaths returns the sorted, present design files among the changed paths.
func designsFromPaths(root string, paths []string) []string {
	var designs []string
	for _, p := range paths {
		if isDesignFile(p) && regularFile(root, p) && !slices.Contains(designs, p) {
			designs = append(designs, p)
		}
	}
	slices.Sort(designs)

	return designs
}

// changedDesigns returns the design files changed against the comparison commit.
func changedDesigns(root, revision string) ([]string, error) {
	if revision == "" {
		return nil, nil
	}
	paths, err := changedPaths(root, revision)
	if err != nil {
		return nil, err
	}

	return designsFromPaths(root, paths), nil
}

func checkChangedDesign(root, revision string, c *compiledModel) error {
	paths, err := changedPaths(root, revision)
	if err != nil {
		return err
	}
	var packages []string
	for _, p := range paths {
		if changedPackage(p) {
			pkg := packageOf(p)
			if _, exists := c.Packages[pkg]; exists {
				packages = append(packages, pkg)
			}
		}
	}
	contracts := changedOwners(paths, c.Contracts)
	policies := changedOwners(paths, c.Policies)
	designs := designsFromPaths(root, paths)
	if len(packages) == 0 && len(contracts) == 0 && len(policies) == 0 && len(designs) == 0 {
		return nil
	}
	if len(designs) == 0 {
		return fmt.Errorf(
			"modeled changes need a design; fix: add %s/<issue>.json and run the pre-code design check",
			designDir,
		)
	}
	checked, err := checkDesigns(root, revision, c, designs)
	if err != nil {
		return err
	}
	var covered design
	for _, d := range checked {
		covered.Changes = append(covered.Changes, d.Changes...)
		covered.Contracts = append(covered.Contracts, d.Contracts...)
		covered.Policies = append(covered.Policies, d.Policies...)
	}

	return checkDesignCoverage(covered, packages, contracts, policies)
}

// checkDesigns checks each design against the model in the commit that last changed it.
// At least one design must also pass the checkout model, so a design must cover each later model change.
func checkDesigns(root, revision string, c *compiledModel, paths []string) ([]design, error) {
	designs := make([]design, 0, len(paths))
	matched := false
	var mismatch error
	for _, p := range paths {
		var d design
		if err := loadJSON(root, p, &d); err != nil {
			return nil, err
		}
		snapshot, err := designModel(root, revision, p, c)
		if err != nil {
			return nil, err
		}
		if err := checkDesign(snapshot, d); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if !matched {
			if snapshot == c {
				matched = true
			} else if mismatch = checkDesign(c, d); mismatch == nil {
				matched = true
			}
		}
		designs = append(designs, d)
	}
	if !matched {
		return nil, fmt.Errorf("no changed design passes the current model; fix: update a design for the model change: %w", mismatch)
	}

	return designs, nil
}

// designModel returns the model in the commit that last changed a design.
// An uncommitted design, or a design that no commit after the comparison changed, uses the checkout model.
func designModel(root, revision, p string, c *compiledModel) (*compiledModel, error) {
	status, err := gitRead(root, "status", "--porcelain", "-z", "--", p)
	if err != nil {
		return nil, err
	}
	if len(status) > 0 {
		return c, nil
	}
	raw, err := gitRead(root, "log", "-1", "--format=%H", revision+"..HEAD", "--", p)
	if err != nil {
		return nil, err
	}
	commit := strings.TrimSpace(string(raw))
	if commit == "" {
		return c, nil
	}
	data, err := commitFile(root, commit, modelFile)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("missing model in the commit that changed %s; fix: commit the model with the design", p)
	}
	var m model
	if err := decode(data, &m); err != nil {
		return nil, fmt.Errorf("model for %s: %w", p, err)
	}

	return compile(m)
}

func checkDesignCoverage(d design, packages, contracts, policies []string) error {
	for _, name := range contracts {
		if !slices.Contains(d.Contracts, name) {
			return fmt.Errorf(
				"contract %q lacks design coverage; fix: declare its compatibility review in %s/<issue>.json",
				name, designDir,
			)
		}
	}
	for _, name := range policies {
		if !slices.Contains(d.Policies, name) {
			return fmt.Errorf(
				"policy %q lacks design coverage; fix: declare its control review in %s/<issue>.json",
				name, designDir,
			)
		}
	}
	for _, pkg := range packages {
		if !slices.ContainsFunc(d.Changes, func(item change) bool { return item.Package == pkg }) {
			return fmt.Errorf(
				"changed package %q lacks design coverage; fix: declare it in %s/<issue>.json before coding",
				pkg, designDir,
			)
		}
	}

	return nil
}

func changedOwners(paths []string, owners map[string]string) []string {
	var changed []string
	for name, p := range owners {
		if slices.Contains(paths, p) {
			changed = append(changed, name)
		}
	}
	slices.Sort(changed)

	return changed
}
