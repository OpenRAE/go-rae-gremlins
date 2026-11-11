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

package workdir

import (
	"errors"
	"testing"
)

type stubCloser struct {
	err    error
	closed bool
}

func (c *stubCloser) Close() error {
	c.closed = true

	return c.err
}

func TestFeatureCloseJoinReturnsCloseErrors(t *testing.T) {
	errCopy := errors.New("copy failed")
	errClose := errors.New("close failed")
	testCases := []struct {
		prior     error
		closeErr  error
		name      string
		wantErrs  []error
		wantNoErr bool
	}{
		{name: "close succeeds after copy succeeds", wantNoErr: true},
		{name: "close succeeds after copy fails", prior: errCopy, wantErrs: []error{errCopy}},
		{name: "close fails after copy succeeds", closeErr: errClose, wantErrs: []error{errClose}},
		{name: "close fails after copy fails", prior: errCopy, closeErr: errClose, wantErrs: []error{errCopy, errClose}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			closer := &stubCloser{err: tc.closeErr}
			err := closeJoin(closer, "destination", tc.prior)

			if !closer.closed {
				t.Error("closeJoin did not close the file")
			}
			if tc.wantNoErr && err != nil {
				t.Errorf("got error %v, want nil", err)
			}
			for _, want := range tc.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("got error %v, want it to wrap %v", err, want)
				}
			}
		})
	}
}
