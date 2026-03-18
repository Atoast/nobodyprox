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
	Tokenize(text string) (ids, mask, starts, ends []int)
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

// Tokenize converts a string into a slice of input IDs with offsets
func (t *WordPieceTokenizer) Tokenize(text string) (ids, mask, starts, ends []int) {
	// A more complex tokenizer would be needed for perfect offsets.
	// For this prototype, we'll estimate them based on space-splitting.
	
	ids = []int{t.vocab["[CLS]"]}
	mask = []int{1}
	starts = []int{0}
	ends = []int{0}

	currentPos := 0
	words := strings.Fields(text)
	for _, word := range words {
		// Find word in original text to get its start offset
		offset := strings.Index(text[currentPos:], word)
		if offset == -1 {
			continue
		}
		wordStart := currentPos + offset
		wordEnd := wordStart + len(word)
		currentPos = wordEnd

		// Only lowercase for vocab lookup if necessary (most bert-base-multilingual are cased)
		subwords := t.wordPiece(word)
		for _, sw := range subwords {
			id := t.vocab["[UNK]"]
			if val, ok := t.vocab[sw]; ok {
				id = val
			}
			ids = append(ids, id)
			mask = append(mask, 1)
			starts = append(starts, wordStart)
			ends = append(ends, wordEnd)
		}
	}

	ids = append(ids, t.vocab["[SEP]"])
	mask = append(mask, 1)
	starts = append(starts, currentPos)
	ends = append(ends, currentPos)

	// Padding/Truncation
	if len(ids) > t.maxLen {
		ids = ids[:t.maxLen]
		mask = mask[:t.maxLen]
		starts = starts[:t.maxLen]
		ends = ends[:t.maxLen]
	}

	for len(ids) < t.maxLen {
		ids = append(ids, t.vocab["[PAD]"])
		mask = append(mask, 0)
		starts = append(starts, 0)
		ends = append(ends, 0)
	}

	return ids, mask, starts, ends
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
			// Try exact match first, then lowercase match
			if _, ok := t.vocab[substr]; ok {
				curSubword = substr
				break
			} else if _, ok := t.vocab[strings.ToLower(substr)]; ok {
				curSubword = strings.ToLower(substr)
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
	clsId  int
	sepId  int
	padId  int
}

// NewBPETokenizer loads a tokenizer.json file and creates a new BPETokenizer
func NewBPETokenizer(path string, maxLen int) (*BPETokenizer, error) {
	tk, err := pretrained.FromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load BPE tokenizer: %v", err)
	}

	getSpecial := func(tokens ...string) int {
		for _, tok := range tokens {
			if id, ok := tk.TokenToId(tok); ok {
				return id
			}
		}
		return 0 // default fallback
	}

	clsId := getSpecial("[CLS]", "<bos>", "<s>")
	sepId := getSpecial("[SEP]", "<eos>", "</s>")
	padId := getSpecial("[PAD]", "<pad>")

	return &BPETokenizer{
		tk:     tk,
		maxLen: maxLen,
		clsId:  clsId,
		sepId:  sepId,
		padId:  padId,
	}, nil
}

func (t *BPETokenizer) Decode(ids []int) string {
	return t.tk.Decode(ids, true)
}

func (t *BPETokenizer) Vocab() map[string]int {
	return nil
}

func (t *BPETokenizer) Tokenize(text string) (ids, mask, starts, ends []int) {
	en, err := t.tk.EncodeSingle(text)
	if err != nil {
		ids = make([]int, t.maxLen)
		mask = make([]int, t.maxLen)
		starts = make([]int, t.maxLen)
		ends = make([]int, t.maxLen)
		return
	}

	needsSpecials := true
	if len(en.Ids) > 0 && en.Ids[0] == t.clsId {
		needsSpecials = false
	}

	if needsSpecials {
		ids = []int{t.clsId}
		ids = append(ids, en.Ids...)
		ids = append(ids, t.sepId)

		mask = []int{1}
		mask = append(mask, en.AttentionMask...)
		mask = append(mask, 1)

		starts = []int{0}
		ends = []int{0}
		for _, offset := range en.Offsets {
			starts = append(starts, offset[0])
			ends = append(ends, offset[1])
		}
		starts = append(starts, len(text))
		ends = append(ends, len(text))
	} else {
		ids = en.Ids
		mask = en.AttentionMask
		for _, offset := range en.Offsets {
			starts = append(starts, offset[0])
			ends = append(ends, offset[1])
		}
	}

	if len(ids) > t.maxLen {
		ids = ids[:t.maxLen]
		mask = mask[:t.maxLen]
		starts = starts[:t.maxLen]
		ends = ends[:t.maxLen]
	}

	for len(ids) < t.maxLen {
		ids = append(ids, t.padId)
		mask = append(mask, 0)
		starts = append(starts, 0)
		ends = append(ends, 0)
	}

	return ids, mask, starts, ends
}
