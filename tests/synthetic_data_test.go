package tests

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

func TestSyntheticLargeData(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	rules := []config.Rule{
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

	// Generate a large block of text
	var sb strings.Builder
	var sensitiveItems []string

	for i := 0; i < 50; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		cpr := generateCPR()
		key := generateOpenAIKey()
		
		sensitiveItems = append(sensitiveItems, email, cpr, key)
		
		sb.WriteString(fmt.Sprintf("User %d info: Email: %s, CPR: %s, Key: %s\n", i, email, cpr, key))
		if i%5 == 0 {
			sb.WriteString("Some neutral text that should not be redacted at all.\n")
		}
	}

	input := sb.String()
	output := engine.Redact(input, "TEST", "TEST-ID")

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

	t.Logf("Successfully redacted a block of %d characters", len(input))
}

func TestConcurrentRedaction(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	rules := []config.Rule{
		{
			Name:    "OPENAI_KEY",
			Pattern: `sk-[a-zA-Z0-9]{48}`,
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

	numWorkers := 10
	numRequestsPerWorker := 20
	done := make(chan bool)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			for j := 0; j < numRequestsPerWorker; j++ {
				email := fmt.Sprintf("worker%d-req%d@example.com", workerID, j)
				key := generateOpenAIKey()
				input := fmt.Sprintf("Worker %d, Request %d: Email: %s, Secret: %s", workerID, j, email, key)
				
				reqID := fmt.Sprintf("W%d-R%d", workerID, j)
				output := engine.Redact(input, "TEST", reqID)
				
				if strings.Contains(output, email) {
					t.Errorf("Output still contains email: %s", email)
				}
				if strings.Contains(output, key) {
					t.Errorf("Output still contains key: %s", key)
				}
				if !strings.Contains(output, "[REDACTED: EMAIL]") || !strings.Contains(output, "[REDACTED: OPENAI_KEY]") {
					t.Errorf("Output missing redaction markers for worker %d, req %d", workerID, j)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all workers to finish
	for i := 0; i < numWorkers; i++ {
		<-done
	}

	t.Logf("Successfully completed %d concurrent redaction requests", numWorkers*numRequestsPerWorker)
}
