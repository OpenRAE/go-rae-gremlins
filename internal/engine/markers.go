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

package engine

import (
	"bytes"
	"slices"
)

// markerWriter scans the output of a test run for markers. It keeps only
// the bytes that can start a marker that the next write completes, so its
// memory does not grow with the output of a mutant. exec.Cmd calls Write
// from one goroutine at a time when Stdout and Stderr are the same
// markerWriter.
type markerWriter struct {
	markers [][]byte
	tail    []byte
	keep    int
	seen    bool
}

// newMarkerWriter returns a markerWriter for markers.
func newMarkerWriter(markers [][]byte) *markerWriter {
	w := &markerWriter{markers: markers}
	for _, marker := range markers {
		w.keep = max(w.keep, len(marker)-1)
	}

	return w
}

// Write scans p, together with the kept end of the earlier output, for the
// markers. It never fails.
func (w *markerWriter) Write(p []byte) (int, error) {
	if w.seen {
		return len(p), nil
	}
	data := slices.Concat(w.tail, p)
	for _, marker := range w.markers {
		if bytes.Contains(data, marker) {
			w.seen = true
		}
	}
	w.tail = append(w.tail[:0], data[max(0, len(data)-w.keep):]...)

	return len(p), nil
}

// found reports whether the output contains a marker.
func (w *markerWriter) found() bool {
	return w.seen
}
