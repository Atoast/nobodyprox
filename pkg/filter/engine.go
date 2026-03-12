package filter

import (
	"crypto/sha256"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
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
	EntityType  string         `yaml:"entity_type"`
	Replacement string         `yaml:"replacement"`
	Action      ActionMode     `yaml:"action"`
	Regex       *regexp.Regexp `yaml:"-"`
}

// Match represents a found piece of sensitive data with its location
type Match struct {
	Start       int
	End         int
	Replacement string
	Type        string
	Original    string
	Action      ActionMode
}

// Engine manages the redaction and pseudonymization process
type Engine struct {
	Rules     []Rule
	NER       NERProvider
	WatchMode bool
	mappings  map[string]string // Original -> Synthetic
	mu        sync.RWMutex
}

// NewEngine creates a new filter engine
func NewEngine(rules []Rule, ner NERProvider, watchMode bool) (*Engine, error) {
	for i := range rules {
		if rules[i].Pattern != "" {
			re, err := regexp.Compile(rules[i].Pattern)
			if err != nil {
				return nil, err
			}
			rules[i].Regex = re
		}
		
		if rules[i].Action == "" {
			rules[i].Action = ActionRedact
		}
		if rules[i].Replacement == "" && rules[i].Action == ActionRedact {
			if rules[i].EntityType != "" {
				rules[i].Replacement = "[REDACTED: " + rules[i].EntityType + "]"
			} else {
				rules[i].Replacement = "[REDACTED: " + rules[i].Name + "]"
			}
		}
	}
	return &Engine{
		Rules:     rules,
		NER:       ner,
		WatchMode: watchMode,
		mappings:  make(map[string]string),
	}, nil
}

// Redact applies all rules (redaction or pseudonymization) to the input string
func (e *Engine) Redact(input, context, reqID string) string {
	if e == nil {
		return input
	}
	return string(e.RedactBytes([]byte(input), context, reqID))
}

// RedactBytes applies all rules and NER detection to the input byte slice using a single-pass replacement
func (e *Engine) RedactBytes(input []byte, context, reqID string) []byte {
	if e == nil {
		return input
	}

	var matches []Match

	// 1. Collect NER Matches
	if e.NER != nil {
		entities, err := e.NER.ExtractEntities(string(input))
		if err == nil {
			for _, ent := range entities {
				// Basic heuristic to skip redacting technical or JSON-structural strings
				trimmed := strings.TrimSpace(ent.Text)
				if len(trimmed) <= 2 || strings.ContainsAny(trimmed, "{}[]\":") {
					continue
				}

				// Find matching rule for this EntityType
				var matchingRule *Rule
				for i := range e.Rules {
					if e.Rules[i].EntityType == string(ent.Type) {
						matchingRule = &e.Rules[i]
						break
					}
				}

				if matchingRule == nil {
					continue
				}

				replacement := matchingRule.Replacement
				if matchingRule.Action == ActionPseudonymize {
					e.mu.Lock()
					if synth, ok := e.mappings[ent.Text]; ok {
						replacement = synth
					} else {
						synth := e.generateSynthetic(ent.Text, string(ent.Type))
						e.mappings[ent.Text] = synth
						replacement = synth
					}
					e.mu.Unlock()
				}

				matches = append(matches, Match{
					Start:       ent.Start,
					End:         ent.End,
					Replacement: replacement,
					Type:        string(ent.Type),
					Original:    ent.Text,
					Action:      matchingRule.Action,
				})
			}
		}
	}

	// 2. Collect Regex Matches
	for _, rule := range e.Rules {
		if rule.Pattern == "" || rule.Regex == nil {
			continue
		}

		locs := rule.Regex.FindAllIndex(input, -1)
		if len(locs) > 0 {
			log.Printf("[%s][%s] Rule %s found %d matches", reqID, context, rule.Name, len(locs))
		}

		for _, loc := range locs {
			val := string(input[loc[0]:loc[1]])
			replacement := rule.Replacement
			
			if rule.Action == ActionPseudonymize {
				e.mu.Lock()
				if synth, ok := e.mappings[val]; ok {
					replacement = synth
				} else {
					synth := e.generateSynthetic(val, rule.Name)
					e.mappings[val] = synth
					replacement = synth
				}
				e.mu.Unlock()
			}

			matches = append(matches, Match{
				Start:       loc[0],
				End:         loc[1],
				Replacement: replacement,
				Type:        rule.Name,
				Original:    val,
				Action:      rule.Action,
			})
		}
	}

	if len(matches) == 0 {
		return input
	}

	// 3. Sort matches by start position descending (to replace from end to start)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start == matches[j].Start {
			return matches[i].End > matches[j].End
		}
		return matches[i].Start > matches[j].Start
	})

	// 4. Handle Overlaps & Execute Replacement
	output := make([]byte, len(input))
	copy(output, input)

	lastStart := len(input) + 1
	for _, m := range matches {
		// Log discovery
		if e.WatchMode {
			log.Printf("[%s][%s][WATCH] Found %s: %s", reqID, context, m.Type, m.Original)
		} else {
			log.Printf("[%s][%s][REDACT] Found %s: %s (Action: %s)", reqID, context, m.Type, m.Original, m.Action)
		}

		if e.WatchMode {
			continue
		}

		// Skip if this match overlaps with a replacement we already made
		if m.End > lastStart {
			continue
		}

		// Backward replacement splice
		newOutput := make([]byte, len(output[:m.Start]))
		copy(newOutput, output[:m.Start])
		newOutput = append(newOutput, []byte(m.Replacement)...)
		newOutput = append(newOutput, output[m.End:]...)
		output = newOutput
		
		lastStart = m.Start
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
