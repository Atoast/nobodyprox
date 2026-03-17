package tests

import (
	"testing"
	"strings"

	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

func TestUserReportedString(t *testing.T) {
	rules := []config.Rule{
		{
			Name:       "ORG_DETECTION",
			EntityType: "ORGANIZATION",
			Action:     "REDACT",
		},
		{
			Name:       "LOC_DETECTION",
			EntityType: "LOCATION",
			Action:     "REDACT",
		},
		{
			Name:       "PERSON_DETECTION",
			EntityType: "PERSON",
			Action:     "PSEUDONYMIZE",
		},
		{
			Name:    "DANISH_CPR",
			Pattern: `\b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-\d{4}\b`,
			Action:  "REDACT",
		},
	}

	input := "My CPR is 010190-1234, jeg bor på østerbro 122. ved siden bor mads hansen østerbrogade. Min onkel hedder bjørn thorsen sørensen andersen, og han har cpr 020219-1245"

	// Helper to get byte offsets
	getByteLoc := func(substr string) (int, int) {
		start := strings.Index(input, substr)
		if start == -1 {
			return -1, -1
		}
		return start, start + len(substr)
	}

	loc1S, loc1E := getByteLoc("østerbro")
	per1S, per1E := getByteLoc("mads hansen")
	loc2S, loc2E := getByteLoc("østerbrogade")
	per2S, per2E := getByteLoc("bjørn thorsen sørensen andersen")

	ner := &mockNERProvider{
		entities: []filter.Entity{
			{Type: "LOCATION", Text: "østerbro", Start: loc1S, End: loc1E},
			{Type: "PERSON", Text: "mads hansen", Start: per1S, End: per1E},
			{Type: "LOCATION", Text: "østerbrogade", Start: loc2S, End: loc2E},
			{Type: "PERSON", Text: "bjørn thorsen sørensen andersen", Start: per2S, End: per2E},
		},
	}

	engine, _ := filter.NewEngine(rules, ner, false)

	// Pass 1: Request
	output1 := engine.Redact(input, "REQ", "REQ-ID")
	
	t.Logf("Output 1: %s", output1)

	// Verify the placeholders are there
	if !strings.Contains(output1, "[REDACTED: DANISH_CPR]") {
		t.Errorf("Missing CPR placeholder in output 1")
	}
	if !strings.Contains(output1, "[REDACTED: LOCATION]") {
		t.Errorf("Missing LOCATION placeholder in output 1")
	}

	// Pass 2: Simulate httpbin response
	// The fix `isInsidePlaceholder` should prevent NER from matching "REDACTED" inside our placeholders
	
	// We need a NEW engine or clear mappings? Mappings are consistent, so it's fine.
	// But we need a NEW mock NER that finds "REDACTED" as an ORG in the output of pass 1.
	
	output1Str := string(output1)
	startRedacted := strings.Index(output1Str, "REDACTED")
	
	recursiveNER := &mockNERProvider{
		entities: []filter.Entity{
			// Simulate finding "REDACTED" as an ORG
			{Type: "ORGANIZATION", Text: "REDACTED", Start: startRedacted, End: startRedacted + 8},
		},
	}
	
	engine2, _ := filter.NewEngine(rules, recursiveNER, false)
	output2 := engine2.Redact(output1Str, "RES", "RES-ID")
	
	t.Logf("Output 2: %s", output2)

	if strings.Contains(output2, "[RED[REDACTED: ORGANIZATION]") {
		t.Errorf("Recursive redaction detected!\nGot: %s", output2)
	}
	
	if output2 != output1Str {
		// Since we only added ONE match to engine2 (the buggy one), output2 should be equal to output1
		// if the fix works.
		t.Errorf("Output changed when it shouldn't have!\nExpected: %s\nGot:      %s", output1Str, output2)
	}
}
