package filter

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
)

// Tokenizer is the interface for all model tokenizers
type Tokenizer interface {
	Tokenize(text string) ([]int, []int)
	Decode(ids []int) string
	Vocab() map[string]int
}

// WordPieceTokenizer represents a simple WordPiece tokenizer for BERT-based models
type WordPieceTokenizer struct {
	vocab    map[string]int
	invVocab map[int]string
	maxLen   int
}

// NewWordPieceTokenizer loads a vocab.txt file and creates a new WordPieceTokenizer
func NewWordPieceTokenizer(vocabPath string, maxLen int) (*WordPieceTokenizer, error) {
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

	return &WordPieceTokenizer{
		vocab:    vocab,
		invVocab: invVocab,
		maxLen:   maxLen,
	}, nil
}

// Tokenize converts a string into a slice of input IDs
func (t *WordPieceTokenizer) Tokenize(text string) ([]int, []int) {
	words := strings.Fields(strings.ToLower(text))
	inputIds := []int{t.vocab["[CLS]"]}
	
	for _, word := range words {
		subwords := t.wordPiece(word)
		for _, sw := range subwords {
			if id, ok := t.vocab[sw]; ok {
				inputIds = append(inputIds, id)
			} else {
				inputIds = append(inputIds, t.vocab["[UNK]"])
			}
		}
	}

	inputIds = append(inputIds, t.vocab["[SEP]"])

	if len(inputIds) > t.maxLen {
		inputIds = inputIds[:t.maxLen]
	}
	
	attentionMask := make([]int, len(inputIds))
	for i := range attentionMask {
		attentionMask[i] = 1
	}

	for len(inputIds) < t.maxLen {
		inputIds = append(inputIds, t.vocab["[PAD]"])
		attentionMask = append(attentionMask, 0)
	}

	return inputIds, attentionMask
}

func (t *WordPieceTokenizer) Decode(ids []int) string {
	var words []string
	for _, id := range ids {
		word, ok := t.invVocab[id]
		if !ok || word == "[CLS]" || word == "[SEP]" || word == "[PAD]" {
			continue
		}
		if strings.HasPrefix(word, "##") {
			if len(words) > 0 {
				words[len(words)-1] += strings.TrimPrefix(word, "##")
			}
		} else {
			words = append(words, word)
		}
	}
	return strings.Join(words, " ")
}

func (t *WordPieceTokenizer) Vocab() map[string]int {
	return t.vocab
}

func (t *WordPieceTokenizer) wordPiece(word string) []string {
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

// BPETokenizer wraps the sugarme/tokenizer for HF compatible BPE
type BPETokenizer struct {
	tk     *tokenizer.Tokenizer
	maxLen int
}

// NewBPETokenizer loads a tokenizer.json file and creates a new BPETokenizer
func NewBPETokenizer(path string, maxLen int) (*BPETokenizer, error) {
	tk, err := pretrained.FromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load BPE tokenizer: %v", err)
	}

	return &BPETokenizer{
		tk:     tk,
		maxLen: maxLen,
	}, nil
}

func (t *BPETokenizer) Decode(ids []int) string {
	// Decode returns string
	res := t.tk.Decode(ids, true)
	return res
}

func (t *BPETokenizer) Vocab() map[string]int {
	return nil
}

func (t *BPETokenizer) Tokenize(text string) ([]int, []int) {
	en, err := t.tk.EncodeSingle(text)
	if err != nil {
		return make([]int, t.maxLen), make([]int, t.maxLen)
	}

	ids := en.Ids
	mask := en.AttentionMask

	if len(ids) > t.maxLen {
		ids = ids[:t.maxLen]
		mask = mask[:t.maxLen]
	}

	for len(ids) < t.maxLen {
		ids = append(ids, 0)
		mask = append(mask, 0)
	}

	return ids, mask
}
