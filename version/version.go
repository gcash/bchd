// Copyright (c) 2013-2014 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package version

import (
	"bytes"
	"fmt"
	"strings"
)

// semanticAlphabet
const semanticAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

// These constants define the application version and follow the semantic
// versioning 2.0.0 spec (http://semver.org/).
const (
	AppMajor uint = 0
	AppMinor uint = 12
	AppPatch uint = 0

	// AppPreRelease MUST only contain characters from semanticAlphabet
	// per the semantic versioning spec.
	AppPreRelease = "beta2"
)

// appBuild is defined as a variable so it can be overridden during the build
// process with '-ldflags "-X main.appBuild foo' if needed.  It MUST only
// contain characters from semanticAlphabet per the semantic versioning spec.
var appBuild string

// String returns the application version as a properly formed string per the
// semantic versioning 2.0.0 spec (http://semver.org/).
func String() string {
	// Start with the major, minor, and patch versions.
	version := fmt.Sprintf("%d.%d.%d", AppMajor, AppMinor, AppPatch)

	// Append pre-release version if there is one.  The hyphen called for
	// by the semantic versioning spec is automatically appended and should
	// not be contained in the pre-release string.  The pre-release version
	// is not appended if it contains invalid characters.
	if AppPreRelease != "" {
		preRelease := normalizeVerString(AppPreRelease)
		if preRelease == AppPreRelease {
			version = fmt.Sprintf("%s-%s", version, preRelease)
		}
	}

	// Append build metadata if there is any.  The plus called for
	// by the semantic versioning spec is automatically appended and should
	// not be contained in the build metadata string.  The build metadata
	// string is not appended if it contains invalid characters.
	if appBuild != "" {
		build := normalizeVerString(appBuild)
		if build == appBuild {
			version = fmt.Sprintf("%s+%s", version, build)
		}
	}

	return version
}

// normalizeVerString returns the passed string stripped of all characters which
// are not valid according to the semantic versioning guidelines for pre-release
// version and build metadata strings.  In particular they MUST only contain
// characters in semanticAlphabet.
func normalizeVerString(str string) string {
	var result bytes.Buffer
	for _, r := range str {
		if strings.ContainsRune(semanticAlphabet, r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}
