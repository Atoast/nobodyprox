package filter

// EntityType represents the type of a found entity (e.g., PERSON, ORG, LOC)
type EntityType string

const (
	EntityPerson       EntityType = "PERSON"
	EntityOrganization EntityType = "ORGANIZATION"
	EntityLocation     EntityType = "LOCATION"
)

// Entity represents a found piece of sensitive data
type Entity struct {
	Type       EntityType
	Text       string
	Start      int
	End        int
	Confidence float64
}

// NERProvider is the interface that all NER backends must implement
type NERProvider interface {
	Name() string
	ExtractEntities(text string) ([]Entity, error)
	Labels() []string
}
