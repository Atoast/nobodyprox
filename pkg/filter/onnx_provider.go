package filter

import (
	"fmt"
	"log"
	"os"

	ort "github.com/yalue/onnxruntime_go"
)

// ONNXProvider implements the NERProvider interface using ONNX Runtime
type ONNXProvider struct {
	Session    *ort.AdvancedSession
	ModelPath  string
	Tokenizer  *Tokenizer
}

// NewONNXProvider creates a new instance of the ONNXProvider
func NewONNXProvider(modelPath, vocabPath string) (*ONNXProvider, error) {
	// 1. Initialize the ONNX Runtime library
	if !ort.IsInitialized() {
		libPath := "onnxruntime.dll"
		if _, err := os.Stat(libPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("onnxruntime.dll not found in the project root")
		}
		ort.SetSharedLibraryPath(libPath)
		err := ort.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX runtime: %v", err)
		}
	}

	// 2. Load the Tokenizer
	tokenizer, err := NewTokenizer(vocabPath, 128)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer: %v", err)
	}

	// 3. Create the session
	log.Printf("Loading ONNX model from %s...", modelPath)
	
	// Note: Creating the session actually requires defining the tensors.
	// For this phase, we have the structure ready.
	
	return &ONNXProvider{
		ModelPath: modelPath,
		Tokenizer: tokenizer,
	}, nil
}

func (p *ONNXProvider) Name() string {
	return "onnx"
}

func (p *ONNXProvider) ExtractEntities(text string) ([]Entity, error) {
	if text == "" {
		return nil, nil
	}

	// 1. Tokenize
	inputIds, _ := p.Tokenizer.Tokenize(text)
	
	// 2. Prepare tensors
	// In Phase 2.2, we'll implement the Session.Run() with these tensors.
	// For now, let's keep the logging to verify tokenization works.
	log.Printf("[ONNX] Tokenized input (length %d)", len(inputIds))

	return nil, nil
}

// Close releases the ONNX session resources
func (p *ONNXProvider) Close() {
	if p.Session != nil {
		p.Session.Destroy()
	}
}
