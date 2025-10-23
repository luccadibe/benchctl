//go:build unit

package execution

import (
	"os"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	// Set a test HOME directory
	testHome := "/home/testuser"
	os.Setenv("HOME", testHome)

	secondEnvVar := "test-value"
	os.Setenv("SECOND_ENV_VAR", secondEnvVar)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no tilde",
			input:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "tilde only",
			input:    "~",
			expected: testHome,
		},
		{
			name:     "tilde with slash",
			input:    "~/",
			expected: testHome + "/",
		},
		{
			name:     "tilde with path",
			input:    "~/.ssh/id_rsa",
			expected: testHome + "/.ssh/id_rsa",
		},
		{
			name:     "tilde with nested path",
			input:    "~/Documents/keys/mykey",
			expected: testHome + "/Documents/keys/mykey",
		},
		{
			name:     "environment variable",
			input:    "$HOME/.ssh/id_rsa",
			expected: testHome + "/.ssh/id_rsa",
		},
		{
			name:     "multiple environment variables",
			input:    "$HOME/.ssh/$SECOND_ENV_VAR",
			expected: testHome + "/.ssh/" + secondEnvVar,
		},
		{
			name:     "mixed tilde and env var",
			input:    "~/.ssh/$KEY_NAME",
			expected: testHome + "/.ssh/", // KEY_NAME not set, so expands to empty string
		},
		{
			name:     "multiple tildes (only first expanded)",
			input:    "~/path/~/another",
			expected: testHome + "/path/~/another",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandTilde(tt.input)
			if result != tt.expected {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandTildeWithEnvironmentVariables(t *testing.T) {
	// Save original environment
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

	// Set test environment
	os.Setenv("HOME", "/home/testuser")
	os.Setenv("KEY_FILE", "/custom/path/key")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "environment variable substitution",
			input:    "$KEY_FILE",
			expected: "/custom/path/key",
		},
		{
			name:     "tilde with environment variable",
			input:    "~/.ssh/$KEY_NAME",
			expected: "/home/testuser/.ssh/", // KEY_NAME not set, so expands to empty string
		},
		{
			name:     "complex path with multiple variables",
			input:    "$HOME/.ssh/${KEY_NAME:-default_key}",
			expected: "/home/testuser/.ssh/", // os.ExpandEnv doesn't support bash parameter expansion, ${KEY_NAME:-default_key} becomes empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandTilde(tt.input)
			if result != tt.expected {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
