package filefilter

import "testing"

func TestGitignoreMatcher_MatchPattern(t *testing.T) {
	m := NewGitignoreMatcher()

	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// Rooted patterns
		{"Rooted build dir", "/build", "build/output.js", true},
		{"Rooted build exact", "/build", "build", true},
		{"Rooted build no match", "/build", "src/build/file.js", false},

		// Globstar patterns
		{"Globstar test files", "**/*.test.js", "src/utils/helper.test.js", true},
		{"Globstar nested test", "**/*.test.js", "components/button/button.test.js", true},
		{"Glob star spec files", "src/**/*.spec.ts", "src/app/service.spec.ts", true},
		{"Globstar deep spec", "src/**/*.spec.ts", "src/components/button/button.spec.ts", true},

		// Bracket expressions
		{"Bracket pyc", "*.py[cod]", "module.pyc", true},
		{"Bracket pyo", "*.py[cod]", "module.pyo", true},
		{"Bracket pyd", "*.py[cod]", "module.pyd", true},
		{"Bracket no match", "*.py[cod]", "module.py", false},

		// Directory patterns
		{"Dir pattern root", "**/temp/", "temp/file.txt", true},
		{"Dir pattern nested", "**/temp/", "src/temp/data.json", true},
		{"Dir pattern deep", "**/temp/", "a/b/c/temp/file.js", true},

		// Simple patterns
		{"Simple extension log", "*.log", "app.log", true},
		{"Simple extension nested", "*.log", "logs/app.log", true},
		{"Simple temp", "*.tmp", "data.tmp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.MatchPattern(tt.pattern, tt.path)
			if got != tt.want {
				regex := m.gitignoreToRegex(tt.pattern)
				t.Errorf("MatchPattern(%q, %q) = %v, want %v\nRegex: %s",
					tt.pattern, tt.path, got, tt.want, regex)
			}
		})
	}
}
