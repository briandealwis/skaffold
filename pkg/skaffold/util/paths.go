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
	"sort"
	"strings"
)

type fileSystem interface {
	// isAbs returns true if the path is absolute. Unlike path.filepath.IsAbs
	// we treat a `\path` without a volume as absolute on Windows.
	isAbs(path string) bool

	// depth returns the number of file system components in the name; the root is 0
	depth(path string) uint

	// isPathSeparator returns true if `c` is a file system path separator
	isPathSeparator(c uint8) bool

	// volLen returns the size of the volume section of a path
	volLen(path string) int
}

func CommonRoots(paths []string, minDepth uint, os string) []string {
	switch os {
	case "windows":
		return roots(paths, minDepth, windowsFileSystem{})
	default:
		return roots(paths, minDepth, unixFileSystem{})
	}
}

// Roots returns a set of common roots of the given paths.  The paths are assumed
// to be clean (as per filepath.Clean).  These common roots will have at least minDepth
// components.  For example,
//   - Roots([/a/b/c, /a/b/d, /c/d/e], 1) = [/a/b, /c/d/e]
//   - Roots([/a/b/c, /c/d/e], 1) = [/a/b/c, /c/d/e]
func roots(paths []string, minDepth uint, fs fileSystem) []string {
	if len(paths) == 0 {
		return nil
	}

	// since the paths are sorted, we do binary search: for each interval, look at the
	// depth of the common prefix of first and last elements.  If not deep enough then
	// split the interval and recurse.
	sort.Strings(paths)

	var roots orderedFileSet
	if !findRoots(paths, 0, len(paths)-1, minDepth, fs, &roots) {
		return nil
	}
	return roots.Files()
}

func findRoots(paths []string, start, end int, minDepth uint, fs fileSystem, roots *orderedFileSet) bool {
	if start == end {
		if fs.depth(paths[start]) < minDepth {
			return false
		}
		roots.Add(paths[start])
		return true
	}
	if common, depth := findCommonPrefix(paths[start], paths[end], fs); common != "" && depth >= minDepth {
		roots.Add(common)
		return true
	}
	// idea: use binary search within `[start,end)` to find item with the common prefix with
	// paths[start] -- which may only include itself
	prefix := ""
	left := start    // start of the search interval
	right := end - 1 // the end of the search interval; we know paths[end] didn't have a common prefix'
	// we iterate, narrowing the interval, until we're looking at one final item
	for left <= right {
		mid := (left + right) / 2
		common, depth := findCommonPrefix(paths[start], paths[mid], fs)
		if common == "" || depth < minDepth {
			right = mid - 1
		} else {
			prefix = common
			left = mid + 1
		}
	}
	if prefix == "" {
		return false
	}
	roots.Add(prefix)
	// assert `right < end` since `right` was initialized to `end-1`
	return findRoots(paths, right+1, end, minDepth, fs, roots)
}

func findCommonPrefix(a, b string, fs fileSystem) (string, uint) {
	if a == b {
		return a, fs.depth(a)
	}
	// swap if necessary to ensure that b is the smaller
	if len(a) < len(b) {
		a, b = b, a
	}
	volLen := fs.volLen(a)
	prefix := ""
	for i := 0; i < len(b); i++ {
		switch {
		case a[i] != b[i]:
			return prefix, fs.depth(prefix)
		case fs.isPathSeparator(b[i]):
			if i == volLen {
				// copy out the root separator (c:\ or \\server\ or /)
				prefix = b[0 : volLen+1]
			} else {
				prefix = b[0:i]
			}
		case i+1 == int(volLen):
			// at least copy out the volume section
			prefix = b[0:volLen]
		}
	}
	// we made to the end so b is a prefix of a; check if the last component matches
	// note: we know that len(a) > len(b)
	if fs.isPathSeparator(a[len(b)]) {
		prefix = b
	}
	return prefix, fs.depth(prefix)
}

// unixFileSystem encodes rules for a Unix file system
type unixFileSystem struct{}

func (fs unixFileSystem) isPathSeparator(c uint8) bool {
	return c == '/'
}

func (fs unixFileSystem) volLen(path string) int {
	return 0
}

func (fs unixFileSystem) isAbs(path string) bool {
	return len(path) > 0 && path[0] == '/'
}

func (fs unixFileSystem) depth(path string) uint {
	if len(path) > 0 && path[0] == '/' {
		return relPathDepth(path[1:], fs)
	}
	return relPathDepth(path, fs)
}

type windowsFileSystem struct{}

func (fs windowsFileSystem) isPathSeparator(c uint8) bool {
	return c == '/' || c == '\\'
}

func (fs windowsFileSystem) isAbs(path string) bool {
	if fs.isUnc(path) {
		return true
	}
	volLen := fs.volLen(path)
	// true if includes root: `c:` and `c:foo` are not absolute as they are relative to working dir on c:)
	return volLen < len(path) && fs.isPathSeparator(path[volLen])
}

func (fs windowsFileSystem) depth(path string) uint {
	volLen := fs.volLen(path)
	if int(volLen) == len(path) {
		return 0
	}
	if fs.isPathSeparator(path[volLen]) {
		return relPathDepth(path[volLen+1:], fs)
	}
	return relPathDepth(path[volLen:], fs)
}

// volLen returns the length of the volume name
func (fs windowsFileSystem) volLen(path string) int {
	switch {
	case fs.hasDriveLetter(path):
		// looks like a drive
		return 2
	case fs.isUnc(path):
		// looks like UNC
		index := strings.IndexByte(path[2:], '\\')
		if index < 0 {
			return len(path)
		}
		return 2 + index
	default:
		return 0
	}
}

func (fs windowsFileSystem) hasDriveLetter(path string) bool {
	return len(path) >= 2 && isAlpha(path[0]) && path[1] == ':'
}

func (fs windowsFileSystem) isUnc(path string) bool {
	return len(path) > 2 && path[0] == '\\' && path[1] == '\\' && path[2] != '\\'
}

// relPathDepth is a helper function that returns the number of path components in the *relative* path
func relPathDepth(path string, fs fileSystem) uint {
	if path == "" {
		return 0
	}
	var depth uint = 1
	for i := 0; i < len(path); i++ {
		if fs.isPathSeparator(path[i]) {
			depth++
		}
	}
	return depth
}

// isAlpha tests that the given character is an alphabetic character in US-ASCII
func isAlpha(c uint8) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
