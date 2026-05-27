//go:build unit

package execution

import (
	"os"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	originalHome := os.Getenv("HOME")
	originalKeyFile := os.Getenv("KEY_FILE")
	defer func() {
		os.Setenv("HOME", originalHome)
		if originalKeyFile != "" {
			os.Setenv("KEY_FILE", originalKeyFile)
		} else {
			os.Unsetenv("KEY_FILE")
		}
	}()

	testHome := "/home/testuser"
	os.Setenv("HOME", testHome)
	os.Setenv("SECOND_ENV_VAR", "test-value")
	os.Setenv("KEY_FILE", "/custom/path/key")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "no tilde", input: "/absolute/path", expected: "/absolute/path"},
		{name: "tilde only", input: "~", expected: testHome},
		{name: "tilde with slash", input: "~/", expected: testHome + "/"},
		{name: "tilde with path", input: "~/.ssh/id_rsa", expected: testHome + "/.ssh/id_rsa"},
		{name: "environment variable", input: "$HOME/.ssh/id_rsa", expected: testHome + "/.ssh/id_rsa"},
		{name: "multiple environment variables", input: "$HOME/.ssh/$SECOND_ENV_VAR", expected: testHome + "/.ssh/test-value"},
		{name: "custom env var", input: "$KEY_FILE", expected: "/custom/path/key"},
		{name: "mixed tilde and unset env var", input: "~/.ssh/$KEY_NAME", expected: testHome + "/.ssh/"},
		{name: "multiple tildes only first expanded", input: "~/path/~/another", expected: testHome + "/path/~/another"},
		{name: "empty string", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExpandTilde(tt.input); got != tt.expected {
				t.Errorf("ExpandTilde(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
