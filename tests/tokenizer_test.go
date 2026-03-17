package tests

import (
	"testing"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

func TestTokenizer(t *testing.T) {
	tokenizer, err := filter.NewWordPieceTokenizer("test_vocab.txt", 16)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	text := "Alice Smith went"
	ids, mask, _, _ := tokenizer.Tokenize(text)

	// Expected: [CLS] (2), alice (5), smith (6), went (7), [SEP] (3), [PAD] (0) ...
	if len(ids) != 16 {
		t.Errorf("expected length 16, got %d", len(ids))
	}
	if len(mask) != 16 {
		t.Errorf("expected mask length 16, got %d", len(mask))
	}

	if ids[0] != 2 { // [CLS]
		t.Errorf("expected index 0 to be [CLS] (2), got %d", ids[0])
	}
	if ids[1] != 5 { // alice
		t.Errorf("expected index 1 to be 'alice' (5), got %d", ids[1])
	}
	if ids[2] != 6 { // smith
		t.Errorf("expected index 2 to be 'smith' (6), got %d", ids[2])
	}
	if ids[3] != 7 { // went
		t.Errorf("expected index 3 to be 'went' (7), got %d", ids[3])
	}
	if ids[4] != 3 { // [SEP]
		t.Errorf("expected index 4 to be [SEP] (3), got %d", ids[4])
	}
	if ids[5] != 0 { // [PAD]
		t.Errorf("expected index 5 to be [PAD] (0), got %d", ids[5])
	}

	// Verify attention mask
	for i := 0; i < 5; i++ {
		if mask[i] != 1 {
			t.Errorf("expected mask[%d] to be 1, got %d", i, mask[i])
		}
	}
	for i := 5; i < 16; i++ {
		if mask[i] != 0 {
			t.Errorf("expected mask[%d] to be 0, got %d", i, mask[i])
		}
	}
}

func TestWordPieceSplitting(t *testing.T) {
	tokenizer, err := filter.NewWordPieceTokenizer("test_vocab.txt", 16)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// "google" is in vocab, "googleoogle" should be split to "google" + "##oogle"
	text := "googleoogle"
	ids, _, _, _ := tokenizer.Tokenize(text)

	// [CLS] (2), google (10), ##oogle (15), [SEP] (3)
	if ids[1] != 10 {
		t.Errorf("expected 'google' (10), got %d", ids[1])
	}
	if ids[2] != 15 {
		t.Errorf("expected '##oogle' (15), got %d", ids[2])
	}
}
