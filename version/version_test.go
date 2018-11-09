package version

import (
	"fmt"
	"testing"
)

func TestString(t *testing.T) {
	base := fmt.Sprintf("%d.%d.%d-%s", AppMajor, AppMinor, AppPatch, AppPreRelease)

	testCases := []struct {
		name     string
		build    string
		expected string
	}{
		{
			name:     "standard-release",
			build:    "",
			expected: base,
		}, {
			name:     "with-build",
			build:    "012-abc",
			expected: base + "+012-abc",
		}, {
			name:     "with-out-of-spec-build",
			build:    "012_abc",
			expected: base,
		},
	}

	for _, tc := range testCases {
		appBuild = tc.build
		v := String()
		if v != tc.expected {
			t.Fatalf("%s: expected %s, got %s", tc.name, tc.expected, v)
		}
	}
}
