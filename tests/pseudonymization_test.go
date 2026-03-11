package tests

import (
	"strings"
	"testing"

	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

func TestPseudonymization(t *testing.T) {
	rules := []filter.Rule{
		{
			Name:   "NAME",
			Pattern: `\b[A-Z][a-z]+\b`, // Simple name pattern
			Action: filter.ActionPseudonymize,
		},
	}

	engine, err := filter.NewEngine(rules, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	input := "Alice went to see Bob. Alice said hello to Bob."
	output1 := engine.Redact(input)
	output2 := engine.Redact(input)

	// Check for consistency
	if output1 != output2 {
		t.Errorf("expected consistent output, got %q and %q", output1, output2)
	}

	// Check if Alice and Bob have DIFFERENT synthetic values
	if strings.Count(output1, "NAME") < 2 {
		t.Errorf("expected multiple pseudonymized names, got %q", output1)
	}

	// Verify that the same name has the same synthetic value
	// Example: [NAME_xxxx] ... [NAME_yyyy]. [NAME_xxxx] ... [NAME_yyyy]
	words := strings.Fields(output1)
	alice1 := words[0] // Alice
	alice2 := words[5] // Alice

	if alice1 != alice2 {
		t.Errorf("expected consistent synthetic value for Alice, got %q and %q", alice1, alice2)
	}

	bob1 := words[4] // Bob.
	bob2 := words[9] // Bob.
	
	// Strip punctuation for comparison
	bob1 = strings.TrimSuffix(bob1, ".")
	bob2 = strings.TrimSuffix(bob2, ".")

	if bob1 != bob2 {
		t.Errorf("expected consistent synthetic value for Bob, got %q and %q", bob1, bob2)
	}
	
	if alice1 == bob1 {
		t.Errorf("expected different synthetic values for Alice and Bob, both got %q", alice1)
	}
}
