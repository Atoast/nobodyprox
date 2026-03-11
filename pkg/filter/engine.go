package filter

import (
	"crypto/sha256"
	"fmt"
	"log"
	"regexp"
	"sync"
)

// ActionMode defines the action to take for a match
type ActionMode string

const (
	ActionRedact       ActionMode = "REDACT"
	ActionPseudonymize ActionMode = "PSEUDONYMIZE"
)

// Rule represents a single redaction or pseudonymization rule
type Rule struct {
	Name        string         `yaml:"name"`
	Pattern     string         `yaml:"pattern"`
	Replacement string         `yaml:"replacement"`
	Action      ActionMode     `yaml:"action"`
	Regex       *regexp.Regexp `yaml:"-"`
}

// Engine manages the redaction and pseudonymization process
type Engine struct {
	Rules    []Rule
	NER      NERProvider
	mappings map[string]string // Original -> Synthetic
	mu       sync.RWMutex
}

// NewEngine creates a new filter engine
func NewEngine(rules []Rule, ner NERProvider) (*Engine, error) {
	for i := range rules {
		re, err := regexp.Compile(rules[i].Pattern)
		if err != nil {
			return nil, err
		}
		rules[i].Regex = re
		if rules[i].Action == "" {
			rules[i].Action = ActionRedact
		}
		if rules[i].Replacement == "" && rules[i].Action == ActionRedact {
			rules[i].Replacement = "[REDACTED: " + rules[i].Name + "]"
		}
	}
	return &Engine{
		Rules:    rules,
		NER:      ner,
		mappings: make(map[string]string),
	}, nil
}

// Redact applies all rules (redaction or pseudonymization) to the input string
func (e *Engine) Redact(input string) string {
	if e == nil {
		return input
	}
	return string(e.RedactBytes([]byte(input)))
}

// RedactBytes applies all rules and NER detection to the input byte slice
func (e *Engine) RedactBytes(input []byte) []byte {
	if e == nil {
		return input
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	output := input

	// 1. Apply NER (Deep Path) if enabled
	if e.NER != nil {
		entities, err := e.NER.ExtractEntities(string(output))
		if err == nil {
			for _, ent := range entities {
				log.Printf("[NER] Found %s: %s", ent.Type, ent.Text)
				if synth, ok := e.mappings[ent.Text]; ok {
					output = []byte(regexp.MustCompile(`\b`+regexp.QuoteMeta(ent.Text)+`\b`).ReplaceAllString(string(output), synth))
					continue
				}

				// Generate a new consistent synthetic value for the entity
				synth := e.generateSynthetic(ent.Text, string(ent.Type))
				e.mappings[ent.Text] = synth
				output = []byte(regexp.MustCompile(`\b`+regexp.QuoteMeta(ent.Text)+`\b`).ReplaceAllString(string(output), synth))
			}
		}
	}

	// 2. Apply Regex rules (Fast Path)
	for _, rule := range e.Rules {
		if rule.Action == ActionRedact {
			output = rule.Regex.ReplaceAll(output, []byte(rule.Replacement))
		} else if rule.Action == ActionPseudonymize {
			output = rule.Regex.ReplaceAllFunc(output, func(match []byte) []byte {
				val := string(match)
				if synth, ok := e.mappings[val]; ok {
					return []byte(synth)
				}

				// Generate a new consistent synthetic value
				synth := e.generateSynthetic(val, rule.Name)
				e.mappings[val] = synth
				return []byte(synth)
			})
		}
	}
	return output
}

// generateSynthetic creates a consistent synthetic value for a given input
func (e *Engine) generateSynthetic(original, ruleName string) string {
	h := sha256.New()
	h.Write([]byte(original))
	hash := fmt.Sprintf("%x", h.Sum(nil))[:8]
	return fmt.Sprintf("[%s_%s]", ruleName, hash)
}

// ClearMappings resets the pseudonymization table
func (e *Engine) ClearMappings() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mappings = make(map[string]string)
}
