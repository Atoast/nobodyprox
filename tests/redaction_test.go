package tests

import (
	"testing"

	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

func TestRedactionEngine(t *testing.T) {
	rules := []filter.Rule{
		{
			Name:    "OPENAI_KEY",
			Pattern: `sk-[a-zA-Z0-9]{48}`,
		},
		{
			Name:    "DANISH_CPR",
			Pattern: `\b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-\d{4}\b`,
		},
		{
			Name:    "EMAIL",
			Pattern: `\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`,
		},
	}

	engine, err := filter.NewEngine(rules)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Redact OpenAI Key",
			input:    "My key is sk-1234567890abcdef1234567890abcdef1234567890abcdef and I am happy.",
			expected: "My key is [REDACTED: OPENAI_KEY] and I am happy.",
		},
		{
			name:     "Redact Danish CPR",
			input:    "My CPR is 010190-1234, please don't share it.",
			expected: "My CPR is [REDACTED: DANISH_CPR], please don't share it.",
		},
		{
			name:     "Redact Email",
			input:    "Contact me at user@example.com for more info.",
			expected: "Contact me at [REDACTED: EMAIL] for more info.",
		},
		{
			name:     "Multiple Redactions",
			input:    "User user@example.com with CPR 010190-1234 uses key sk-1234567890abcdef1234567890abcdef1234567890abcdef.",
			expected: "User [REDACTED: EMAIL] with CPR [REDACTED: DANISH_CPR] uses key [REDACTED: OPENAI_KEY].",
		},
		{
			name:     "No Redaction Needed",
			input:    "This is a safe string with no sensitive data.",
			expected: "This is a safe string with no sensitive data.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := engine.Redact(tt.input)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
