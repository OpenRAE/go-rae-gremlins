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

package log_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/spf13/viper"

	"github.com/OpenRAE/go-rae-gremlins/internal/configuration"
	"github.com/OpenRAE/go-rae-gremlins/internal/log"
)

func TestUninitialised(t *testing.T) {
	// Bind buffers first, so that Reset must detach them.
	out := &bytes.Buffer{}
	eOut := &bytes.Buffer{}
	log.Init(out, eOut)
	log.Reset()

	stdout, stderr := captureStdio(t, func() {
		log.Infof("%s", "test")
		log.Infoln("test")
		log.Errorf("%s", "test")
		log.Errorln("test")
	})

	if got := out.String(); got != "" {
		t.Errorf("out after Reset: want empty, got %q", got)
	}
	if got := eOut.String(); got != "" {
		t.Errorf("eOut after Reset: want empty, got %q", got)
	}
	if stdout != "" {
		t.Errorf("os.Stdout of uninitialised logger: want empty, got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("os.Stderr of uninitialised logger: want empty, got %q", stderr)
	}
}

// captureStdio runs fn with os.Stdout and os.Stderr redirected to temporary
// files and returns what fn wrote to each.
func captureStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	dir := t.TempDir()
	outFile := createFile(t, dir, "stdout")
	errFile := createFile(t, dir, "stderr")

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, errFile
	func() {
		defer func() { os.Stdout, os.Stderr = origOut, origErr }()
		fn()
	}()

	return readClosed(t, outFile), readClosed(t, errFile)
}

// createFile creates a temporary file in dir and closes it at test cleanup
// if readClosed has not closed it.
func createFile(t *testing.T, dir, pattern string) *os.File {
	t.Helper()

	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("create %s file: %v", pattern, err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close %s: %v", f.Name(), err)
		}
	})

	return f
}

// readClosed closes f and returns its content.
func readClosed(t *testing.T, f *os.File) string {
	t.Helper()

	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", f.Name(), err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("read %s: %v", f.Name(), err)
	}

	return string(data)
}

func TestLogInfo(t *testing.T) {
	out := &bytes.Buffer{}
	log.Init(out, &bytes.Buffer{})

	t.Run("Infof", func(t *testing.T) {
		defer out.Reset()

		log.Infof("test %d", 1)

		got := out.String()

		want := "test 1"
		if got != want {
			t.Errorf("want %q, got %q", want, got)
		}
	})

	t.Run("Infoln", func(t *testing.T) {
		defer out.Reset()

		log.Infoln("test test")

		got := out.String()

		want := "test test\n"
		if got != want {
			t.Errorf("want %q, got %q", want, got)
		}
	})
	log.Reset()
}

func TestLogError(t *testing.T) {
	out := &bytes.Buffer{}
	eOut := &bytes.Buffer{}
	log.Init(out, eOut)
	defer log.Reset()

	t.Run("Errorf", func(t *testing.T) {
		defer out.Reset()
		defer eOut.Reset()

		log.Errorf("test %d", 1)

		got := eOut.String()

		want := "ERROR: test 1"
		if got != want {
			t.Errorf("want %q, got %q", want, got)
		}

		got = out.String()
		if got != "" {
			t.Errorf("expected out to be empty, got %s", got)
		}
	})

	t.Run("Errorln", func(t *testing.T) {
		defer out.Reset()
		defer eOut.Reset()

		log.Errorln("test test")

		got := eOut.String()

		want := "ERROR: test test\n"
		if got != want {
			t.Errorf("want %q, got %q", want, got)
		}

		got = out.String()
		if got != "" {
			t.Errorf("expected out to be empty, got %s", got)
		}
	})
}

func TestRegressionSilentModeFromEnvironment(t *testing.T) {
	configuration.Reset()
	t.Cleanup(configuration.Reset)
	if err := configuration.Init([]string{t.TempDir()}); err != nil {
		t.Fatalf("configuration.Init error = %v, want nil", err)
	}
	t.Setenv("GREMLINS_SILENT", "true")
	sOut := &bytes.Buffer{}
	log.Init(sOut, &bytes.Buffer{})
	t.Cleanup(log.Reset)

	log.Infoln("test")

	if got := sOut.String(); got != "" {
		t.Errorf("silent output = %q, want empty", got)
	}
}

func TestSilentMode(t *testing.T) {
	viper.Set("silent", true)
	defer viper.Reset()

	sOut := &bytes.Buffer{}
	defer sOut.Reset()
	eOut := &bytes.Buffer{}
	defer eOut.Reset()
	log.Init(sOut, eOut)
	defer log.Reset()

	log.Infof("%s", "test")
	log.Infoln("test")
	log.Errorf("%s\n", "test")
	log.Errorln("test")

	if sOut.String() != "" {
		t.Errorf("expected empty string")
	}
	if eOut.String() != "ERROR: test\nERROR: test\n" {
		t.Log(eOut.String())
		t.Errorf("expected errors to be reported")
	}
}
