package filter

import (
	"fmt"
	"github.com/jdkato/prose/v2"
)

// ProseProvider implements the NERProvider interface using the Prose library
type ProseProvider struct {
	model *prose.Document
}

// NewProseProvider creates a new instance of the ProseProvider
func NewProseProvider() (*ProseProvider, error) {
	// Prose doesn't need much initialization, but we can verify it's working
	return &ProseProvider{}, nil
}

func (p *ProseProvider) Name() string {
	return "prose"
}

func (p *ProseProvider) ExtractEntities(text string) ([]Entity, error) {
	doc, err := prose.NewDocument(text)
	if err != nil {
		return nil, fmt.Errorf("failed to create prose document: %v", err)
	}

	var entities []Entity
	for _, ent := range doc.Entities() {
		entityType := EntityPerson
		switch ent.Label {
		case "PERSON":
			entityType = EntityPerson
		case "GPE", "LOC":
			entityType = EntityLocation
		case "ORG":
			entityType = EntityOrganization
		default:
			// For now, only track common ones
			continue
		}

		entities = append(entities, Entity{
			Type:       entityType,
			Text:       ent.Text,
			Confidence: 1.0, // Prose doesn't provide confidence scores
		})
	}

	return entities, nil
}
