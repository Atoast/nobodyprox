package tests

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

func generateCPR() string {
	day := rand.Intn(28) + 1
	month := rand.Intn(12) + 1
	year := rand.Intn(99)
	suffix := rand.Intn(9999)
	return fmt.Sprintf("%02d%02d%02d-%04d", day, month, year, suffix)
}

func generateOpenAIKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 48)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return "sk-" + string(b)
}

func TestDanishSyntheticData(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	rules := []filter.Rule{
		{
			Name:    "OPENAI_KEY",
			Pattern: `sk-[a-zA-Z0-9]{48}`,
		},
		{
			Name:    "DANISH_CPR",
			Pattern: `\b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-\d{4}\b`,
		},
		{
			Name:    "EMAIL",
			Pattern: `\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`,
		},
	}

	engine, err := filter.NewEngine(rules, nil, false)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Names and emails used in the script for Danish
	daNames := []string{"Mette Jensen", "Lars Nielsen", "Hanne Pedersen", "Kristian Poulsen", "Sofie Andersen"}
	daEmails := []string{"mette.j@firma.dk", "lars.n@mail.dk", "hanne.p@tjeneste.dk"}

	// Generate a block of Danish text
	var sb strings.Builder
	var sensitiveItems []string

	for i := 0; i < 20; i++ {
		name := daNames[rand.Intn(len(daNames))]
		email := daEmails[rand.Intn(len(daEmails))]
		cpr := generateCPR()
		key := generateOpenAIKey()
		
		sensitiveItems = append(sensitiveItems, email, cpr, key)
		
		sb.WriteString(fmt.Sprintf("Bruger %d info: Navn: %s, E-mail: %s, CPR: %s, Nøgle: %s\n", i, name, email, cpr, key))
		sb.WriteString("Her er noget dansk tekst som ikke skal fjernes.\n")
	}

	input := sb.String()
	output := engine.Redact(input, "TEST", "DA-ID")

	// Verify that none of the original sensitive items exist in the output
	for _, item := range sensitiveItems {
		if strings.Contains(output, item) {
			t.Errorf("Output still contains sensitive item: %s", item)
		}
	}

	// Verify that redaction markers are present
	if !strings.Contains(output, "[REDACTED: EMAIL]") {
		t.Errorf("Output missing EMAIL redaction markers")
	}
	if !strings.Contains(output, "[REDACTED: DANISH_CPR]") {
		t.Errorf("Output missing DANISH_CPR redaction markers")
	}
	if !strings.Contains(output, "[REDACTED: OPENAI_KEY]") {
		t.Errorf("Output missing OPENAI_KEY redaction markers")
	}

	t.Logf("Successfully redacted a block of %d characters of Danish text", len(input))
}
