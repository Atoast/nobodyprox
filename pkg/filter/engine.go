package filter

import (
	"crypto/sha256"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/event"
)

// ActionMode defines the action to take for a match
type ActionMode string

const (
	ActionRedact       ActionMode = "REDACT"
	ActionPseudonymize ActionMode = "PSEUDONYMIZE"
)

// Match represents a found piece of sensitive data with its location
type Match struct {
	Start       int
	End         int
	Replacement string
	Type        string
	Original    string
	Action      string
}

// Engine manages the redaction and pseudonymization process
type Engine struct {
	Rules     []config.Rule
	NER       NERProvider
	WatchMode bool
	mappings  map[string]string // Original -> Synthetic
	mu        sync.RWMutex
}

// NewEngine creates a new filter engine
func NewEngine(rules []config.Rule, ner NERProvider, watchMode bool) (*Engine, error) {
	for i := range rules {
		if rules[i].Pattern != "" {
			re, err := regexp.Compile(rules[i].Pattern)
			if err != nil {
				return nil, err
			}
			rules[i].Regex = re
		}
		
		if rules[i].Action == "" {
			rules[i].Action = string(ActionRedact)
		}
		if rules[i].Replacement == "" && rules[i].Action == string(ActionRedact) {
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

// isInsidePlaceholder checks if the given byte range is part of an existing placeholder or structural JSON
func (e *Engine) isInsidePlaceholder(input []byte, start, end int) bool {
	s := string(input)
	
	// 1. Check if we are inside brackets [ ... ]
	// We look for the most immediate brackets surrounding this range
	lastOpen := strings.LastIndex(s[:start+1], "[")
	if lastOpen != -1 {
		nextClose := strings.Index(s[lastOpen:], "]")
		if nextClose != -1 {
			nextClose += lastOpen
			if start >= lastOpen && end <= nextClose+1 {
				content := s[lastOpen+1 : nextClose]
				// If it's our placeholder format
				if strings.HasPrefix(content, "REDACTED: ") || (strings.Contains(content, "_") && len(content)-strings.LastIndex(content, "_")-1 == 8) {
					return true
				}
			}
		}
	}

	// 2. Check if we are redacting part of a JSON escape sequence or key
	// If the text contains \u or is part of a key
	raw := s[start:end]
	if strings.Contains(raw, "\\u") || strings.ContainsAny(raw, "{}\":") {
		return true
	}

	return false
}

// Redact applies all rules (redaction or pseudonymization) to the input string
func (e *Engine) Redact(input, context, reqID string) string {
	if e == nil {
		return input
	}
	return string(e.RedactBytes([]byte(input), context, reqID))
}

// DebugRedact returns the input text with sensitive items tagged, e.g., "Hello <PERSON:Alice>"
func (e *Engine) DebugRedact(input string) string {
	if e == nil {
		return input
	}

	var matches []Match
	data := []byte(input)

	// 1. Collect NER Matches
	if e.NER != nil {
		entities, err := e.NER.ExtractEntities(input)
		if err == nil {
			for _, ent := range entities {
				matches = append(matches, Match{
					Start:       ent.Start,
					End:         ent.End,
					Replacement: fmt.Sprintf("<%s:%s>", ent.Type, ent.Text),
				})
			}
		}
	}

	// 2. Collect Regex Matches
	for _, rule := range e.Rules {
		if rule.Pattern == "" || rule.Regex == nil {
			continue
		}
		locs := rule.Regex.FindAllIndex(data, -1)
		for _, loc := range locs {
			val := string(data[loc[0]:loc[1]])
			matches = append(matches, Match{
				Start:       loc[0],
				End:         loc[1],
				Replacement: fmt.Sprintf("<%s:%s>", rule.Name, val),
			})
		}
	}

	if len(matches) == 0 {
		return input
	}

	// 3. Sort ASCENDING for forward construction
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start < matches[j].Start
	})

	var result strings.Builder
	lastIdx := 0
	for _, m := range matches {
		if m.Start < lastIdx {
			continue
		}
		result.WriteString(input[lastIdx:m.Start])
		result.WriteString(m.Replacement)
		lastIdx = m.End
	}
	result.WriteString(input[lastIdx:])

	return result.String()
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

				// Guard against recursive redaction or structural damage
				if e.isInsidePlaceholder(input, ent.Start, ent.End) {
					continue
				}

				// If in WatchMode, log EVERYTHING found by NER to help author rules
				if e.WatchMode {
					log.Printf("[%s][%s][WATCH] Found NER %s: %s", reqID, context, ent.Type, ent.Text)
					event.GlobalBus.Publish(event.Event{
						Type:  event.TypeDetection,
						ReqID: reqID,
						Data: event.DetectionData{
							Context:  context,
							RuleType: string(ent.Type),
							Original: ent.Text,
							Action:   "WATCH",
						},
					})
				}

				// Find matching rule for this EntityType
				var matchingRule *config.Rule
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
				if matchingRule.Action == string(ActionPseudonymize) {
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

				// Log discovery
				if !e.WatchMode {
					log.Printf("[%s][%s][NER] Found %s: %s (Action: %s)", reqID, context, ent.Type, ent.Text, matchingRule.Action)
					
					// Publish Event
					event.GlobalBus.Publish(event.Event{
						Type:  event.TypeDetection,
						ReqID: reqID,
						Data: event.DetectionData{
							Context:  context,
							RuleType: string(ent.Type),
							Original: ent.Text,
							Action:   matchingRule.Action,
						},
					})
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
		if len(locs) > 0 && !e.WatchMode {
			log.Printf("[%s][%s] Rule %s found %d matches", reqID, context, rule.Name, len(locs))
		}

		for _, loc := range locs {
			// Guard against recursive redaction or structural damage
			if e.isInsidePlaceholder(input, loc[0], loc[1]) {
				continue
			}

			val := string(input[loc[0]:loc[1]])
			replacement := rule.Replacement
			action := rule.Action
			if e.WatchMode {
				action = "WATCH"
			}
			
			if rule.Action == string(ActionPseudonymize) && !e.WatchMode {
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

			// Log discovery
			if e.WatchMode {
				log.Printf("[%s][%s][WATCH] Found Rule %s: %s", reqID, context, rule.Name, val)
			} else {
				log.Printf("[%s][%s][REDACT] Found %s: %s (Action: %s)", reqID, context, rule.Name, val, rule.Action)
			}

			// Publish Event
			event.GlobalBus.Publish(event.Event{
				Type:  event.TypeDetection,
				ReqID: reqID,
				Data: event.DetectionData{
					Context:  context,
					RuleType: rule.Name,
					Original: val,
					Action:   action,
				},
			})

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

	// 3. Normalize Matches (Handle whitespace-inclusive tokens)
	for i := range matches {
		m := &matches[i]
		raw := string(input[m.Start:m.End])
		trimmedLeft := strings.TrimLeft(raw, " \n\r\t")
		if trimmedLeft == "" {
			continue
		}
		
		m.Start += (len(raw) - len(trimmedLeft))
		trimmedAll := strings.TrimRight(trimmedLeft, " \n\r\t")
		m.End = m.Start + len(trimmedAll)
		m.Original = trimmedAll
	}

	// 4. Sort matches by start position ascending
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start == matches[j].Start {
			return matches[i].End > matches[j].End
		}
		return matches[i].Start < matches[j].Start
	})

	// 5. Execute Replacement (only if not in watch mode)
	if e.WatchMode {
		return input
	}

	// Stable replacement using a buffer and original offsets
	var result []byte
	lastIdx := 0

	for _, m := range matches {
		// Skip if this match overlaps with the end of the previous replacement
		if m.Start < lastIdx {
			continue
		}

		// Append text from last match to this match
		result = append(result, input[lastIdx:m.Start]...)
		
		// Append replacement
		result = append(result, []byte(m.Replacement)...)
		
		lastIdx = m.End
	}

	// Append remaining text
	if lastIdx < len(input) {
		result = append(result, input[lastIdx:]...)
	}

	return result
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

func (e *Engine) Labels() []string {
	if e.NER == nil {
		return nil
	}
	return e.NER.Labels()
}
