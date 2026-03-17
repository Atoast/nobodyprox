package tests

import (
	"testing"

	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

type mockNERProvider struct {
	entities []filter.Entity
}

func (m *mockNERProvider) Name() string { return "mock" }
func (m *mockNERProvider) ExtractEntities(text string) ([]filter.Entity, error) {
	return m.entities, nil
}
func (m *mockNERProvider) Labels() []string { return []string{"LOCATION", "PERSON"} }

func TestWhitespacePreservation(t *testing.T) {
	rules := []config.Rule{
		{
			Name:       "LOC_DETECTION",
			EntityType: "LOCATION",
			Action:     "REDACT",
		},
	}

	tests := []struct {
		name     string
		input    string
		entities []filter.Entity
		expected string
	}{
		{
			name:  "Space before entity",
			input: "jeg bor på østerbro",
			entities: []filter.Entity{
				{
					Type:  "LOCATION",
					Text:  " østerbro",
					Start: 11, // Byte offset of the space
					End:   21, // Byte offset of the end
				},
			},
			// Current behavior will produce "jeg bor på[REDACTED: LOCATION]"
			expected: "jeg bor på [REDACTED: LOCATION]",
		},
		{
			name:  "No space included",
			input: "jeg bor på østerbro",
			entities: []filter.Entity{
				{
					Type:  "LOCATION",
					Text:  "østerbro",
					Start: 12, // Byte offset of 'ø'
					End:   21,
				},
			},
			expected: "jeg bor på [REDACTED: LOCATION]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ner := &mockNERProvider{entities: tt.entities}
			engine, _ := filter.NewEngine(rules, ner, false)
			actual := engine.Redact(tt.input, "TEST", "W-ID")
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
