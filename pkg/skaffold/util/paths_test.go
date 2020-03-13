/*
Copyright 2020 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/skaffold/testutil"
)

func TestRelPathDepth(t *testing.T) {
	tests := []struct {
		path     string
		fs       fileSystem
		expected uint
	}{
		{"", unixFileSystem{}, 0},
		{"a", unixFileSystem{}, 1},
		{"a/b", unixFileSystem{}, 2},
		{"a/b/c", unixFileSystem{}, 3},
		{"apple/brocolli/citrus", unixFileSystem{}, 3},

		{"", windowsFileSystem{}, 0},
		{"a", windowsFileSystem{}, 1},
		{"a\\b", windowsFileSystem{}, 2},
		{"a\\b\\c", windowsFileSystem{}, 3},
		{"apple\\brocolli\\citrus", windowsFileSystem{}, 3},
		{"a/b", windowsFileSystem{}, 2},
		{"a/b/c", windowsFileSystem{}, 3},
		{"apple/brocolli/citrus", windowsFileSystem{}, 3},
	}

	for _, test := range tests {
		testutil.Run(t, test.path, func(t *testutil.T) {
			depth := relPathDepth(test.path, test.fs)
			t.CheckDeepEqual(test.expected, depth)
		})
	}
}

func TestFindCommonPrefix(t *testing.T) {
	tests := []struct {
		a      string
		b      string
		fs     fileSystem
		result string
		depth  uint
	}{
		{"/a/b/c", "/a/b/c", unixFileSystem{}, "/a/b/c", 3},
		{"/a/b/c", "/a/b", unixFileSystem{}, "/a/b", 2},
		{"/a/b/c", "/a/c", unixFileSystem{}, "/a", 1},
		{"/a/b/c", "/b/c", unixFileSystem{}, "/", 0},
		{"/a/b/c", "/a/c", unixFileSystem{}, "/a", 1},
		{"/a/b/c", "/b/c", unixFileSystem{}, "/", 0},
		{"/apple/b/c", "/app/b", unixFileSystem{}, "/", 0},
		{"/apple/b/c", "apple/b", unixFileSystem{}, "", 0},
		{"/apple", "/app", unixFileSystem{}, "/", 0},
		{"a/b/c", "a/b", unixFileSystem{}, "a/b", 2},
		{"a/b/c", "a/b/c", unixFileSystem{}, "a/b/c", 3},
		{"a/b/c", "b/c", unixFileSystem{}, "", 0},
		{"a", "b", unixFileSystem{}, "", 0},
		{"", "", unixFileSystem{}, "", 0},
		{"", "/a", unixFileSystem{}, "", 0},
		{"", "a", unixFileSystem{}, "", 0},

		{`\a\b\c`, `\a\b\c`, windowsFileSystem{}, `\a\b\c`, 3},
		{`\a\b\c`, `\a\b`, windowsFileSystem{}, `\a\b`, 2},
		{`\a\b\c`, `\a\c`, windowsFileSystem{}, `\a`, 1},
		{`\a\b\c`, `\b\c`, windowsFileSystem{}, `\`, 0},
		{`\a\b\c`, `\a\c`, windowsFileSystem{}, `\a`, 1},
		{`\a\b\c`, `\b\c`, windowsFileSystem{}, `\`, 0},
		{`\apple\b\c`, `\app\b`, windowsFileSystem{}, `\`, 0},
		{`\apple\b\c`, `apple\b`, windowsFileSystem{}, "", 0},
		{`\apple`, `\app`, windowsFileSystem{}, `\`, 0},
		{`a\b\c`, `a\b`, windowsFileSystem{}, `a\b`, 2},
		{`a\b\c`, `a\b\c`, windowsFileSystem{}, `a\b\c`, 3},
		{`a\b\c`, `b\c`, windowsFileSystem{}, "", 0},
		{`c:\a\b\c`, `c:`, windowsFileSystem{}, `c:`, 0},
		{`c:\a\b\c`, `c:\b\c`, windowsFileSystem{}, `c:\`, 0},
		{`c:\a\b\c`, `c:\a\b`, windowsFileSystem{}, `c:\a\b`, 2},
		{`c:\a\b\c`, `b\c`, windowsFileSystem{}, "", 0},
		{`c:\a\b\c`, `c:a\b\c`, windowsFileSystem{}, "c:", 0},
		{`\\server\vol\a\b\c`, `\\server\vol\a\c`, windowsFileSystem{}, `\\server\vol\a`, 2},
		{`\\server\vol\a\b\c`, `\\server\vol\b\c`, windowsFileSystem{}, `\\server\vol`, 1},
		{`\\server\vol\a\b\c`, `b\c`, windowsFileSystem{}, "", 0},
		{`\\server\vol\a\b\c`, `\a\b\c`, windowsFileSystem{}, "", 0},
		{`\\server\vol1\a\b\c`, `\\server\vol2\a`, windowsFileSystem{}, `\\server\`, 0},
		{"", "", windowsFileSystem{}, "", 0},
		{"", `\a`, windowsFileSystem{}, "", 0},
		{"", "a", windowsFileSystem{}, "", 0},
		{"", "c:a", windowsFileSystem{}, "", 0},
		{"", `c:\a`, windowsFileSystem{}, "", 0},
		{"", `\\server\vol\a`, windowsFileSystem{}, "", 0},
	}
	for _, test := range tests {
		testutil.Run(t, test.a+", "+test.b, func(t *testutil.T) {
			prefix, depth := findCommonPrefix(test.a, test.b, test.fs)
			t.CheckDeepEqual(test.result, prefix)
			t.CheckDeepEqual(test.depth, depth)
		})
	}
}

// testFileSystem encodes rules for a test file system
type testFileSystem struct{}

func (fs testFileSystem) isPathSeparator(c uint8) bool {
	return c == '|'
}

func (fs testFileSystem) isAbs(path string) bool {
	return len(path) > 0 && path[0] == '|'
}

func (fs testFileSystem) volLen(path string) int {
	return 0
}

func (fs testFileSystem) depth(path string) uint {
	if len(path) > 0 && path[0] == '|' {
		return relPathDepth(path[1:], fs)
	}
	return relPathDepth(path, fs)
}

func TestRoots(t *testing.T) {
	tests := []struct {
		paths    []string
		minDepth uint
		result   []string
	}{
		{nil, 0, nil},
		{[]string{"|"}, 0, []string{"|"}},
		{[]string{"|"}, 1, nil},
		{[]string{"|a|b", "|c|d"}, 0, []string{"|"}},
		{[]string{"|a|b", "|c|d"}, 1, []string{"|a|b", "|c|d"}},
		{[]string{"|a|b", "|c|d"}, 2, []string{"|a|b", "|c|d"}},
		{[]string{"|a|b", "|c|d"}, 3, nil},
		{[]string{"|a|b|c", "|a|b|d", "|a|b|e"}, 0, []string{"|a|b"}},
		{[]string{"|a|b|c", "|a|b|d", "|a|b|e"}, 1, []string{"|a|b"}},
		{[]string{"|a|b|c", "|a|b|d", "|a|b|e"}, 2, []string{"|a|b"}},
		{[]string{"|a|b|c", "|a|b|d", "|a|b|e"}, 3, []string{"|a|b|c", "|a|b|d", "|a|b|e"}},
		{[]string{"|a|b|c", "|a|b|d", "|a|b|e"}, 4, nil},
		{[]string{"a|b", "|c|d"}, 0, []string{"a|b", "|c|d"}},
		{[]string{"a|b", "|c|d"}, 1, []string{"a|b", "|c|d"}},
		{[]string{"a|b", "|c|d"}, 2, []string{"a|b", "|c|d"}},
		{[]string{"a|b", "|c|d"}, 3, nil},
		{[]string{"a|b", "a|c", "c|d"}, 1, []string{"a", "c|d"}},
		{[]string{"a|b", "a|c", "c|d"}, 2, []string{"a|b", "a|c", "c|d"}},
		{[]string{"a|b", "c|d", "d|f", "c|e", "a|c", "d|g"}, 1, []string{"a", "c", "d"}},
	}

	for _, test := range tests {
		testutil.Run(t, strings.Join(test.paths, ", ")+", "+strconv.Itoa(int(test.minDepth)), func(t *testutil.T) {
			roots := roots(test.paths, test.minDepth, testFileSystem{})
			t.CheckDeepEqual(test.result, roots)
		})
	}
}

func TestIsAbs(t *testing.T) {
	tests := []struct {
		path   string
		fs     fileSystem
		result bool
	}{
		{"/a/b/c", unixFileSystem{}, true},
		{"/a", unixFileSystem{}, true},
		{"a/b/c", unixFileSystem{}, false},
		{"a", unixFileSystem{}, false},
		{"", unixFileSystem{}, false},

		{`\a\b\c`, windowsFileSystem{}, true},
		{`\apple`, windowsFileSystem{}, true},
		{`a\b\c`, windowsFileSystem{}, false},
		{`a`, windowsFileSystem{}, false},
		{`c:\a\b`, windowsFileSystem{}, true},
		{`c:\`, windowsFileSystem{}, true},
		{`c:`, windowsFileSystem{}, false},
		{`c:foo`, windowsFileSystem{}, false}, // relative to working dir on c:
		{`c:\a\b\`, windowsFileSystem{}, true},
		{`\\server\vol\a\b\c`, windowsFileSystem{}, true},
		{`\\server\vol\b`, windowsFileSystem{}, true},
		{`\\server\vol\`, windowsFileSystem{}, true},
		{`\\server\vol`, windowsFileSystem{}, true},
		{`\\server\`, windowsFileSystem{}, true},
		{`\\server`, windowsFileSystem{}, true},
		{"", windowsFileSystem{}, false},
	}
	for _, test := range tests {
		testutil.Run(t, test.path, func(t *testutil.T) {
			result := test.fs.isAbs(test.path)
			t.CheckDeepEqual(test.result, result)
		})
	}
}
