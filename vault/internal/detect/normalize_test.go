package detect

import (
	"reflect"
	"sort"
	"testing"
)

func TestNormalizeProjectName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"korva", "korva"},
		{"My Project", "my-project"},
		{"my-project", "my-project"},
		{"my_project", "my-project"},
		{"MyProject", "myproject"},
		{"my   project   name", "my-project-name"},
		{"---my--project---", "my-project"},
		{"home-api.v2", "home-api-v2"},
		{"falabella/financiero", "falabella-financiero"},
		{"123-numeric", "123-numeric"},
		{"!@#$%^&", ""},
		{"  leading and trailing  ", "leading-and-trailing"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := NormalizeProjectName(tc.in); got != tc.want {
				t.Errorf("NormalizeProjectName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFindSimilarProjects(t *testing.T) {
	known := []string{"korva", "my-project", "my_project", "home-api", "Home-API", "myproject", "checkout"}

	tests := []struct {
		name   string
		target string
		want   []string
	}{
		// "my-project" and "my_project" both normalize to "my-project"
		{"separator variants match", "my-project", []string{"my_project"}},
		{"separator variants match (reverse)", "my_project", []string{"my-project"}},
		// "MYPROJECT" normalizes to "myproject" (no insertion of separators on case)
		{"capitalization only", "MYPROJECT", []string{"myproject"}},
		// "HOME_API" → "home-api"; matches both "home-api" and "Home-API".
		{"capitalization + separator", "HOME_API", []string{"home-api", "Home-API"}},
		{"no match", "totally-unique", nil},
		{"empty target", "", nil},
		{"empty after normalize", "!!!", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FindSimilarProjects(tc.target, known)
			// FindSimilarProjects preserves the order of `known`, but the test
			// reads better with a stable comparison.
			sort.Strings(got)
			sortedWant := append([]string(nil), tc.want...)
			sort.Strings(sortedWant)
			if !reflect.DeepEqual(got, sortedWant) {
				t.Errorf("FindSimilarProjects(%q) = %v, want %v", tc.target, got, tc.want)
			}
		})
	}
}
