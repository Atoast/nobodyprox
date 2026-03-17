package tests

import (
	"testing"

	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

type recursiveMockNER struct{}

func (m *recursiveMockNER) Name() string { return "recursive-mock" }
func (m *recursiveMockNER) ExtractEntities(text string) ([]filter.Entity, error) {
	var ents []filter.Entity
	// Simulating NER finding "REDACTED" as an ORGANIZATION
	// in the string "[REDACTED: DANISH_CPR]"
	start := -1
	for j := 0; j < len(text)-8; j++ {
		if text[j:j+8] == "REDACTED" {
			start = j
			break
		}
	}
	if start != -1 {
		ents = append(ents, filter.Entity{
			Type:  "ORGANIZATION",
			Text:  "REDACTED",
			Start: start,
			End:   start + 8,
		})
	}
	return ents, nil
}
func (m *recursiveMockNER) Labels() []string { return []string{"ORGANIZATION"} }

func TestRecursiveRedaction(t *testing.T) {
	rules := []config.Rule{
		{
			Name:       "ORG_DETECTION",
			EntityType: "ORGANIZATION",
			Action:     "REDACT",
		},
		{
			Name:    "DANISH_CPR",
			Pattern: `\b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-\d{4}\b`,
			Action:  "REDACT",
		},
	}

	ner := &recursiveMockNER{}
	engine, _ := filter.NewEngine(rules, ner, false)

	// Pass 1: Request
	input := "My CPR is 010190-1234"
	output1 := engine.Redact(input, "REQ", "REQ-ID")
	expected1 := "My CPR is [REDACTED: DANISH_CPR]"
	if output1 != expected1 {
		t.Fatalf("Pass 1 failed: expected %q, got %q", expected1, output1)
	}

	// Pass 2: Response (the bug happens here)
	output2 := engine.Redact(output1, "RES", "RES-ID")
	// If the bug exists, output2 will be something like "My CPR is [[REDACTED: ORGANIZATION]: DANISH_CPR]"
	if output2 != output1 {
		t.Errorf("Pass 2 failed (Recursive Redaction detected): expected %q, got %q", output1, output2)
	}
}
