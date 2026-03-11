package tests

import (
	"testing"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

func TestProseProvider(t *testing.T) {
	p, err := filter.NewProseProvider()
	if err != nil {
		t.Fatalf("Failed to create Prose provider: %v", err)
	}

	text := "Alice Smith went to visit Google in London."
	entities, err := p.ExtractEntities(text)
	if err != nil {
		t.Fatalf("Failed to extract entities: %v", err)
	}

	foundAlice := false
	foundGoogle := false
	foundLondon := false

	for _, ent := range entities {
		if ent.Text == "Alice Smith" && ent.Type == filter.EntityPerson {
			foundAlice = true
		}
		if ent.Text == "Google" && ent.Type == filter.EntityOrganization {
			foundGoogle = true
		}
		if ent.Text == "London" && ent.Type == filter.EntityLocation {
			foundLondon = true
		}
	}

	if !foundAlice {
		t.Errorf("Did not find Alice Smith as PERSON")
	}
	if !foundGoogle {
		t.Errorf("Did not find Google as ORGANIZATION")
	}
	if !foundLondon {
		t.Errorf("Did not find London as LOCATION")
	}
}
