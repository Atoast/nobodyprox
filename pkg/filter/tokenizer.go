package filter

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Tokenizer represents a simple WordPiece tokenizer for BERT-based models
type Tokenizer struct {
	vocab map[string]int
	invVocab map[int]string
	maxLen int
}

// NewTokenizer loads a vocab.txt file and creates a new Tokenizer
func NewTokenizer(vocabPath string, maxLen int) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open vocab file: %v", err)
	}
	defer f.Close()

	vocab := make(map[string]int)
	invVocab := make(map[int]string)
	scanner := bufio.NewScanner(f)
	index := 0
	for scanner.Scan() {
		word := scanner.Text()
		vocab[word] = index
		invVocab[index] = word
		index++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Tokenizer{
		vocab:    vocab,
		invVocab: invVocab,
		maxLen:   maxLen,
	}, nil
}

// Tokenize converts a string into a slice of input IDs
func (t *Tokenizer) Tokenize(text string) ([]int, []int) {
	// Simple whitespace-based pre-tokenization
	words := strings.Fields(strings.ToLower(text))
	
	// Start with [CLS] token
	inputIds := []int{t.vocab["[CLS]"]}
	
	for _, word := range words {
		// WordPiece algorithm
		subwords := t.wordPiece(word)
		for _, sw := range subwords {
			if id, ok := t.vocab[sw]; ok {
				inputIds = append(inputIds, id)
			} else {
				inputIds = append(inputIds, t.vocab["[UNK]"])
			}
		}
	}

	// Add [SEP] token
	inputIds = append(inputIds, t.vocab["[SEP]"])

	// Truncate or pad to maxLen
	if len(inputIds) > t.maxLen {
		inputIds = inputIds[:t.maxLen]
	}
	
	// Create attention mask
	attentionMask := make([]int, len(inputIds))
	for i := range attentionMask {
		attentionMask[i] = 1
	}

	// Padding
	for len(inputIds) < t.maxLen {
		inputIds = append(inputIds, t.vocab["[PAD]"])
		attentionMask = append(attentionMask, 0)
	}

	return inputIds, attentionMask
}

func (t *Tokenizer) wordPiece(word string) []string {
	var subwords []string
	start := 0
	for start < len(word) {
		end := len(word)
		var curSubword string
		for start < end {
			substr := word[start:end]
			if start > 0 {
				substr = "##" + substr
			}
			if _, ok := t.vocab[substr]; ok {
				curSubword = substr
				break
			}
			end--
		}

		if curSubword == "" {
			subwords = append(subwords, "[UNK]")
			break
		}
		subwords = append(subwords, curSubword)
		start = end
	}
	return subwords
}
