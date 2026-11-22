/*
 * Copyright 2022 The Gremlins Authors
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

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/OpenRAE/go-rae-gremlins/cmd/internal/flags"
	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/coverage"
	"github.com/OpenRAE/go-rae-gremlins/internal/diff"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine"
	"github.com/OpenRAE/go-rae-gremlins/internal/engine/workdir"
	"github.com/OpenRAE/go-rae-gremlins/internal/exclusion"
	"github.com/OpenRAE/go-rae-gremlins/internal/gomodule"
	"github.com/OpenRAE/go-rae-gremlins/internal/log"
	"github.com/OpenRAE/go-rae-gremlins/internal/mutator"
	"github.com/OpenRAE/go-rae-gremlins/internal/report"
)

type unleashCmd struct {
	cmd *cobra.Command
}

const (
	commandName = "unleash"

	// buildCacheDir is the name of the Go build cache directory of a run in
	// its work directory.
	buildCacheDir = "gocache"

	paramDiff               = "diff"
	paramBuildTags          = "tags"
	paramCoverPackages      = "coverpkg"
	paramDryRun             = "dry-run"
	paramOutputStatuses     = "output-statuses"
	paramOutputDiffStatuses = "output-diff-statuses"
	paramOutput             = "output"
	paramIntegrationMode    = "integration"
	paramExcludeFiles       = "exclude-files"
	paramTestCPU            = "test-cpu"
	paramWorkers            = "workers"
	paramTimeoutCoefficient = "timeout-coefficient"
	paramTimeoutMargin      = "timeout-safety-margin"
	paramMemoryBudget       = "memory-budget"

	// Thresholds.
	paramThresholdEfficacy  = "threshold-efficacy"
	paramThresholdMCoverage = "threshold-mcover"
)

func newUnleashCmd(ctx context.Context) (*unleashCmd, error) {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [path]", commandName),
		Aliases: []string{"run", "r"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Unleash the gremlins",
		Long:    longExplainer(),
		RunE:    runUnleash(ctx),
	}

	if err := setFlagsOnCmd(cmd); err != nil {
		return nil, err
	}

	return &unleashCmd{cmd: cmd}, nil
}

func longExplainer() string {
	return heredoc.Doc(`
		Unleashes the gremlins and performs mutation testing on a Go module. It works by
		first gathering the coverage of the test suite and then analysing the source
		code to look for supported mutants.

		Unleash only tests covered mutants, since it doesn't make sense to test mutants
		that no test case is able to catch.

		In 'dry-run' mode, unleash gathers the coverage and analyses the source code,
		but it does not run the tests of each mutant. The coverage run still runs the
		tests.

		Thresholds are configurable quality gates that make gremlins exit with an error
		if those values are not met. Efficacy is the percent of KILLED mutants over
		the total KILLED and LIVED mutants. Mutant coverage is the percent of total
		KILLED + LIVED mutants, over the total mutants.
	`)
}

func runUnleash(ctx context.Context) func(cmd *cobra.Command, args []string) error {
	return func(_ *cobra.Command, args []string) error {
		if err := configuration.Validate(); err != nil {
			return err
		}
		log.Infoln("Starting...")
		path, err := modulePath(args)
		if err != nil {
			return err
		}
		mod, err := gomodule.Init(path)
		if err != nil {
			return fmt.Errorf("not in a Go module: %w", err)
		}

		workDir, err := newWorkDir()
		if err != nil {
			return fmt.Errorf("impossible to create the workdir: %w", err)
		}
		defer cleanUp(workDir)
		if err := checkWorkDir(workDir, mod.Root); err != nil {
			return err
		}

		stopNotice := context.AfterFunc(ctx, func() {
			log.Infof("\nShutting down gracefully...\n")
		})
		defer stopNotice()

		results, err := run(ctx, mod, workDir)
		if ctx.Err() != nil {
			return fmt.Errorf("mutation testing cancelled: %w", context.Cause(ctx))
		}
		if err != nil {
			return err
		}

		return report.Do(results)
	}
}

// modulePath returns the absolute path in args, or the current directory when
// args is empty.
func modulePath(args []string) (string, error) {
	if len(args) > 0 {
		path, err := filepath.Abs(args[0])
		if err != nil {
			return "", fmt.Errorf("impossible to resolve the module path: %w", err)
		}

		return path, nil
	}
	path, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("impossible to get the current directory: %w", err)
	}

	return path, nil
}

// newWorkDir makes the temporary work directory of a run and returns its
// absolute path. The go commands of the run run in other directories, so a
// relative path in the work directory would point to a different directory.
func newWorkDir() (string, error) {
	tmp, err := filepath.Abs(os.TempDir())
	if err != nil {
		return "", err
	}

	return os.MkdirTemp(tmp, "gremlins-")
}

// checkWorkDir returns an error when workDir is inside the module root. The
// copies of the module in workDir would then copy workDir into themselves.
// Both paths are absolute. It resolves symbolic links, so an alias of a
// directory in the module also fails.
func checkWorkDir(workDir, root string) error {
	realWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		return fmt.Errorf("impossible to resolve the workdir: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("impossible to resolve the module root: %w", err)
	}
	if rel, err := filepath.Rel(realRoot, realWorkDir); err == nil && filepath.IsLocal(rel) {
		return fmt.Errorf("the workdir %s is inside the module %s: use a temporary directory outside the module",
			workDir, root)
	}

	return nil
}

// runBuildCache returns the Go build cache of a run with the work directory
// workDir. A GOCACHE environment variable keeps its value. Without it, the
// run uses a new cache in workDir, so the run does not add to the default
// build cache, and the removal of workDir removes the cache.
func runBuildCache(workDir string) string {
	if cache := os.Getenv("GOCACHE"); cache != "" {
		return cache
	}

	return filepath.Join(workDir, buildCacheDir)
}

func cleanUp(wd string) {
	if err := os.RemoveAll(wd); err != nil {
		log.Errorf("impossible to remove temporary folder: %s\n\t%s", err, wd)
	}
}

// run runs the mutation testing of mod in workDir, an absolute path. All go
// commands of the run use the build cache of runBuildCache. The caller
// removes workDir.
func run(ctx context.Context, mod gomodule.GoModule, workDir string) (report.Results, error) {
	fDiff, err := diff.New(filepath.Join(mod.Root, mod.CallingDir))
	if err != nil {
		return report.Results{}, err
	}

	buildCache := runBuildCache(workDir)
	c := coverage.New(workDir, buildCache, mod)

	exclude, err := exclusion.New()
	if err != nil {
		return report.Results{}, err
	}

	cProfile, err := c.Run(ctx)
	if err != nil {
		return report.Results{}, fmt.Errorf("failed to gather coverage: %w", err)
	}

	wdDealer := workdir.NewCachedDealer(workDir, mod.Root)
	defer wdDealer.Clean()

	jDealer, err := engine.NewExecutorDealer(mod, wdDealer, buildCache, cProfile)
	if err != nil {
		return report.Results{}, err
	}

	codeData := engine.CodeData{
		Cov:       cProfile.Profile,
		Diff:      fDiff,
		Exclusion: exclude,
	}

	mut := engine.New(mod, codeData, jDealer)

	return mut.Run(ctx)
}

func setFlagsOnCmd(cmd *cobra.Command) error {
	cmd.Flags().SortFlags = false
	cmd.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		from := []string{".", "_"}
		to := "-"
		for _, sep := range from {
			name = strings.ReplaceAll(name, sep, to)
		}

		return pflag.NormalizedName(name)
	})

	for _, f := range unleashFlags() {
		if err := flags.Set(cmd, f); err != nil {
			return err
		}
	}

	return nil
}

// unleashFlags returns the flags of the unleash command with their
// configuration keys and defaults. The mutant type flags come last.
func unleashFlags() []*flags.Flag {
	fls := []*flags.Flag{
		{
			Name: paramDryRun, CfgKey: configuration.UnleashDryRunKey, Shorthand: "d", DefaultV: false,
			Usage: "find mutants but do not run their tests (the coverage run still runs the tests)",
		},
		{
			Name: paramOutputStatuses, CfgKey: configuration.UnleashOutputStatusesKey, Shorthand: "S", DefaultV: "",
			Usage: "print only statuses from this flag, allowed values - 'lctmkvsre'",
		},
		{
			Name: paramOutputDiffStatuses, CfgKey: configuration.UnleashOutputDiffStatusesKey, DefaultV: "",
			Usage: "print diff for mutants with these statuses, allowed values - 'lk'",
		},
		{
			Name: paramBuildTags, CfgKey: configuration.UnleashTagsKey, Shorthand: "t", DefaultV: "",
			Usage: "a comma-separated list of build tags",
		},
		{
			Name: paramCoverPackages, CfgKey: configuration.UnleashCoverPkgKey, DefaultV: "",
			Usage: "a comma-separated list of package patterns",
		},
		{
			Name: paramDiff, CfgKey: configuration.UnleashDiffRef, Shorthand: "D", DefaultV: "",
			Usage: "diff branch or commit",
		},
		{
			Name: paramOutput, CfgKey: configuration.UnleashOutputKey, Shorthand: "o", DefaultV: "",
			Usage: "set the output file for machine readable results",
		},
		{
			Name: paramIntegrationMode, CfgKey: configuration.UnleashIntegrationMode, Shorthand: "i", DefaultV: false,
			Usage: "makes Gremlins run the complete test suite for each mutation",
		},
		{
			Name: paramExcludeFiles, CfgKey: configuration.UnleashExcludeFiles, Shorthand: "E", DefaultV: []string(nil),
			Usage: "exclude files from Gremlins run by filepath regexp",
		},
		{
			Name: paramThresholdEfficacy, CfgKey: configuration.UnleashThresholdEfficacyKey, DefaultV: float64(0),
			Usage: "threshold for code-efficacy percent",
		},
		{
			Name: paramThresholdMCoverage, CfgKey: configuration.UnleashThresholdMCoverageKey, DefaultV: float64(0),
			Usage: "threshold for mutant-coverage percent",
		},
		{
			Name: paramWorkers, CfgKey: configuration.UnleashWorkersKey, DefaultV: 0,
			Usage: "the number of workers to use in mutation testing",
		},
		{
			Name: paramTestCPU, CfgKey: configuration.UnleashTestCPUKey, DefaultV: 0,
			Usage: "the number of CPUs to allow each test run to use",
		},
		{
			Name: paramTimeoutCoefficient, CfgKey: configuration.UnleashTimeoutCoefficientKey, DefaultV: 0,
			Usage: "the coefficient by which the timeout is increased",
		},
		{
			Name: paramTimeoutMargin, CfgKey: configuration.UnleashTimeoutSafetyMarginKey, DefaultV: 0,
			Usage: "the multiplier of the longest passing test run of a package that gives its timeout (default 2)",
		},
		{
			Name: paramMemoryBudget, CfgKey: configuration.UnleashMemoryBudgetKey, DefaultV: "",
			Usage: "the memory that all the mutant test runs can use together, such as 512M or 2G " +
				"(default: the smaller of 1G and half of the available memory)",
		},
	}

	return append(fls, mutantTypeFlags()...)
}

// mutantTypeFlags returns one enable flag for each mutant type.
func mutantTypeFlags() []*flags.Flag {
	fls := make([]*flags.Flag, 0, len(mutator.Types))
	for _, mt := range mutator.Types {
		name := mt.String()
		param := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		fls = append(fls, &flags.Flag{
			Name:     param,
			CfgKey:   configuration.MutantTypeEnabledKey(mt),
			DefaultV: configuration.IsDefaultEnabled(mt),
			Usage:    fmt.Sprintf("enable %q mutants", name),
		})
	}

	return fls
}
