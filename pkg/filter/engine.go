package filter

import (
	"regexp"
	"sync"
)

// Rule represents a single redaction rule
type Rule struct {
	Name        string         `yaml:"name"`
	Pattern     string         `yaml:"pattern"`
	Replacement string         `yaml:"replacement"`
	Regex       *regexp.Regexp `yaml:"-"`
}

// Engine manages the redaction process
type Engine struct {
	Rules []Rule
	mu    sync.RWMutex
}

// NewEngine creates a new redaction engine
func NewEngine(rules []Rule) (*Engine, error) {
	for i := range rules {
		re, err := regexp.Compile(rules[i].Pattern)
		if err != nil {
			return nil, err
		}
		rules[i].Regex = re
		if rules[i].Replacement == "" {
			rules[i].Replacement = "[REDACTED: " + rules[i].Name + "]"
		}
	}
	return &Engine{Rules: rules}, nil
}

// Redact applies all rules to the input string
func (e *Engine) Redact(input string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	output := input
	for _, rule := range e.Rules {
		output = rule.Regex.ReplaceAllString(output, rule.Replacement)
	}
	return output
}

// RedactBytes applies all rules to the input byte slice
func (e *Engine) RedactBytes(input []byte) []byte {
	e.mu.RLock()
	defer e.mu.RUnlock()

	output := input
	for _, rule := range e.Rules {
		output = rule.Regex.ReplaceAll(output, []byte(rule.Replacement))
	}
	return output
}
