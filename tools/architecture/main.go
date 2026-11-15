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

// Command architecture checks repository boundaries and proposed designs.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/OpenRAE/go-rae-gremlins/internal/log"
)

const modelFile = ".architecture/model.json"
const baselineFile = ".architecture/baseline.json"
const designDir = ".architecture/designs"

func main() {
	log.Init(os.Stdout, os.Stderr)
	base := flag.String("base", "origin/dev", "Git comparison commit or ref (empty means first adoption)")
	proposed := flag.String("design", "", "check a single proposed design file instead of source")
	changed := flag.Bool("design-changed", false, "check the design files changed against -base, then exit")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Errorln("unexpected arguments; fix: use -base, -design, or -design-changed")
		os.Exit(1)
	}
	if err := run(*base, *proposed, *changed); err != nil {
		log.Errorln(err)
		os.Exit(1)
	}
}

func run(base, proposed string, changed bool) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get checkout: %w", err)
	}
	var m model
	if err := loadJSON(root, modelFile, &m); err != nil {
		return err
	}
	c, err := compile(m)
	if err != nil {
		return err
	}
	data, err := readLocal(root, modelFile)
	if err != nil {
		return err
	}
	if proposed != "" {
		return checkProposedFile(root, c, data, proposed)
	}
	if changed {
		return checkProposedChanges(root, base, c, data)
	}

	return checkRepository(root, base, c)
}

func checkProposedFile(root string, c *compiledModel, modelData []byte, path string) error {
	var d design
	if err := loadJSON(root, path, &d); err != nil {
		return err
	}
	if err := checkDesign(c, d); err != nil {
		return err
	}
	logDesignPass(c, modelData, path, d.Issue)

	return nil
}

func checkProposedChanges(root, base string, c *compiledModel, modelData []byte) error {
	revision, err := resolveRevision(root, base)
	if err != nil {
		return err
	}
	designs, err := changedDesigns(root, revision)
	if err != nil {
		return err
	}
	if len(designs) == 0 {
		log.Infof("No changed design under %s; add %s/<issue>.json before coding.\n", designDir, designDir)

		return nil
	}
	for _, path := range designs {
		var d design
		if err := loadJSON(root, path, &d); err != nil {
			return err
		}
		if err := checkDesign(c, d); err != nil {
			return err
		}
		logDesignPass(c, modelData, path, d.Issue)
	}

	return nil
}

func logDesignPass(c *compiledModel, modelData []byte, path string, issue int) {
	ids := make([]string, 0, len(c.byID))
	for id := range c.byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	log.Infof(
		"Design %s (#%d) passed: model SHA256 %x; rules %q. Review the contract and trust assertions against the code.\n",
		path,
		issue,
		sha256.Sum256(modelData),
		strings.Join(ids, ", "),
	)
}

func checkRepository(root, base string, c *compiledModel) error {
	var candidate baseline
	if err := loadJSON(root, baselineFile, &candidate); err != nil {
		return err
	}
	if err := validateBaseline(candidate, c); err != nil {
		return err
	}
	target, revision, err := targetBaseline(root, base)
	if err != nil {
		return err
	}
	if err := validateBaseline(target, c); err != nil {
		return err
	}
	findings, err := scan(root, c)
	if err != nil {
		return err
	}
	for _, f := range findings {
		log.Infoln(formatFinding(f, c))
	}
	if err := ratchet(findings, candidate.Findings, target.Findings); err != nil {
		return err
	}
	if revision != "" {
		if err := checkChangedDesign(root, revision, c); err != nil {
			return err
		}
	}
	log.Infof("Architecture passed: %d baseline findings; comparison %q.\n", len(findings), revision)

	return nil
}
