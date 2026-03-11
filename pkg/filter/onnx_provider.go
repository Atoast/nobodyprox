package filter

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

// ONNXProvider implements the NERProvider interface using ONNX Runtime
type ONNXProvider struct {
	Session    *ort.AdvancedSession
	ModelPath  string
	ConfigPath string
	Tokenizer  Tokenizer
	Labels     map[int]string
	
	// Configured input names
	inputNames []string
	
	// Pre-allocated tensors for efficiency
	inputIdsTensor      *ort.Tensor[int64]
	attentionMaskTensor *ort.Tensor[int64]
	tokenTypeIdsTensor  *ort.Tensor[int64]
	outputTensor        *ort.Tensor[float32]
}

// NewONNXProvider creates a new instance of the ONNXProvider
func NewONNXProvider(modelPath, vocabPath, configPath, onnxURL, modelURL, vocabURL, configURL string, labels map[int]string) (*ONNXProvider, error) {
	// 1. Bootstrap missing resources
	if err := BootstrapONNX(modelPath, vocabPath, configPath, onnxURL, modelURL, vocabURL, configURL); err != nil {
		return nil, fmt.Errorf("failed to bootstrap ONNX resources: %v", err)
	}

	// 2. Initialize the ONNX Runtime library
	if !ort.IsInitialized() {
		libPath := "onnxruntime.dll"
		absPath, err := filepath.Abs(libPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %v", libPath, err)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("onnxruntime.dll not found at %s", absPath)
		}
		ort.SetSharedLibraryPath(absPath)
		err = ort.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX runtime: %v", err)
		}
	}

	// 3. Load the Tokenizer
	var tokenizer Tokenizer
	var err error
	if strings.HasSuffix(vocabPath, ".json") {
		tokenizer, err = NewBPETokenizer(vocabPath, 128)
	} else {
		tokenizer, err = NewWordPieceTokenizer(vocabPath, 128)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer: %v", err)
	}

	// 4. Load Labels (Auto-discovery if empty)
	if len(labels) == 0 && configPath != "" {
		log.Printf("Labels empty, attempting auto-discovery from %s", configPath)
		labels, err = parseLabels(configPath)
		if err != nil {
			log.Printf("Warning: failed to parse labels from config: %v", err)
		}
	}

	// 5. Discover model inputs/outputs
	inputs, _, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get model info: %v", err)
	}

	inputNames := make([]string, len(inputs))
	for i, in := range inputs {
		inputNames[i] = in.Name
	}

	// 6. Pre-allocate tensors
	maxLen := int64(128)
	inputShape := ort.NewShape(1, maxLen)
	
	inputIdsTensor, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		return nil, err
	}
	attentionMaskTensor, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		inputIdsTensor.Destroy()
		return nil, err
	}

	var tokenTypeIdsTensor *ort.Tensor[int64]
	var inputValues []ort.Value
	inputValues = append(inputValues, inputIdsTensor, attentionMaskTensor)

	// Only allocate token_type_ids if the model expects it
	hasTokenTypeIds := false
	for _, name := range inputNames {
		if name == "token_type_ids" {
			hasTokenTypeIds = true
			break
		}
	}

	if hasTokenTypeIds {
		tokenTypeIdsTensor, err = ort.NewEmptyTensor[int64](inputShape)
		if err != nil {
			inputIdsTensor.Destroy()
			attentionMaskTensor.Destroy()
			return nil, err
		}
		inputValues = append(inputValues, tokenTypeIdsTensor)
	}

	// Calculate numLabels from mapping
	maxLabelIdx := 0
	for idx := range labels {
		if idx > maxLabelIdx {
			maxLabelIdx = idx
		}
	}
	numLabels := maxLabelIdx + 1
	outputShape := ort.NewShape(1, maxLen, int64(numLabels))
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		inputIdsTensor.Destroy()
		attentionMaskTensor.Destroy()
		if tokenTypeIdsTensor != nil {
			tokenTypeIdsTensor.Destroy()
		}
		return nil, err
	}

	// 7. Create the advanced session
	log.Printf("Loading ONNX model from %s...", modelPath)
	session, err := ort.NewAdvancedSession(modelPath,
		inputNames,
		[]string{"logits"},
		inputValues,
		[]ort.Value{outputTensor},
		nil)
	if err != nil {
		inputIdsTensor.Destroy()
		attentionMaskTensor.Destroy()
		if tokenTypeIdsTensor != nil {
			tokenTypeIdsTensor.Destroy()
		}
		outputTensor.Destroy()
		return nil, fmt.Errorf("failed to create ONNX session: %v", err)
	}
	
	return &ONNXProvider{
		Session:             session,
		ModelPath:           modelPath,
		ConfigPath:          configPath,
		Tokenizer:           tokenizer,
		Labels:              labels,
		inputNames:          inputNames,
		inputIdsTensor:      inputIdsTensor,
		attentionMaskTensor: attentionMaskTensor,
		tokenTypeIdsTensor:  tokenTypeIdsTensor,
		outputTensor:        outputTensor,
	}, nil
}

func (p *ONNXProvider) Name() string {
	return "onnx"
}

func (p *ONNXProvider) ExtractEntities(text string) ([]Entity, error) {
	if text == "" || p.Tokenizer == nil || p.Session == nil {
		return nil, nil
	}

	// 1. Tokenize
	inputIds, attentionMask, starts, ends := p.Tokenizer.Tokenize(text)
	
	// 2. Fill tensors
	idsData := p.inputIdsTensor.GetData()
	maskData := p.attentionMaskTensor.GetData()
	
	for i := range idsData {
		idsData[i] = 0
		maskData[i] = 0
	}

	for i := range inputIds {
		idsData[i] = int64(inputIds[i])
		maskData[i] = int64(attentionMask[i])
	}

	if p.tokenTypeIdsTensor != nil {
		typeData := p.tokenTypeIdsTensor.GetData()
		for i := range typeData {
			typeData[i] = 0
		}
	}

	// 3. Run inference
	err := p.Session.Run()
	if err != nil {
		return nil, err
	}

	logits := p.outputTensor.GetData()
	maxLabelIdx := 0
	for k := range p.Labels {
		if k > maxLabelIdx {
			maxLabelIdx = k
		}
	}
	numLabels := maxLabelIdx + 1
	seqLen := len(inputIds)

	// 4. Post-process logits
	var entities []Entity
	var currentIds []int
	var currentType string
	var currentStart int
	var currentEnd int

	for i := 0; i < seqLen; i++ {
		// Find Argmax
		maxLogit := float32(-1e10)
		maxIdx := 0
		for j := 0; j < numLabels; j++ {
			val := logits[i*numLabels+j]
			if val > maxLogit {
				maxLogit = val
				maxIdx = j
			}
		}

		label := p.Labels[maxIdx]
		baseLabel := strings.TrimPrefix(strings.TrimPrefix(label, "B-"), "I-")
		isBegin := strings.HasPrefix(label, "B-")
		isInside := strings.HasPrefix(label, "I-")

		if label == "O" || label == "" {
			if len(currentIds) > 0 {
				entities = append(entities, Entity{
					Type:       EntityType(currentType),
					Text:       text[currentStart:currentEnd],
					Start:      currentStart,
					End:        currentEnd,
					Confidence: 1.0,
				})
				currentIds = nil
				currentType = ""
			}
			continue
		}

		// Skip special tokens even if they have an entity label
		if inputIds[i] == 0 || inputIds[i] == 101 || inputIds[i] == 102 { // PAD, CLS, SEP
			continue
		}

		if isBegin || (isInside && baseLabel != currentType) || (currentType != "" && baseLabel != currentType) {
			if len(currentIds) > 0 {
				entities = append(entities, Entity{
					Type:       EntityType(currentType),
					Text:       text[currentStart:currentEnd],
					Start:      currentStart,
					End:        currentEnd,
					Confidence: 1.0,
				})
			}
			currentIds = []int{inputIds[i]}
			currentType = baseLabel
			currentStart = starts[i]
			currentEnd = ends[i]
		} else {
			currentIds = append(currentIds, inputIds[i])
			if currentType == "" {
				currentType = baseLabel
				currentStart = starts[i]
			}
			currentEnd = ends[i]
		}
	}

	if len(currentIds) > 0 {
		entities = append(entities, Entity{
			Type:       EntityType(currentType),
			Text:       text[currentStart:currentEnd],
			Start:      currentStart,
			End:        currentEnd,
			Confidence: 1.0,
		})
	}

	return entities, nil
}

func parseLabels(configPath string) (map[int]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config struct {
		Id2Label map[string]string `json:"id2label"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	labels := make(map[int]string)
	for k, v := range config.Id2Label {
		idx, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		
		// Map standard HF/NER tags to our internal EntityType strings
		upperVal := strings.ToUpper(v)
		label := upperVal
		
		// Map common aliases
		if strings.Contains(upperVal, "PER") {
			label = strings.Replace(upperVal, "PER", "PERSON", 1)
		} else if strings.Contains(upperVal, "ORG") {
			label = strings.Replace(upperVal, "ORG", "ORGANIZATION", 1)
		} else if strings.Contains(upperVal, "LOC") || strings.Contains(upperVal, "GPE") {
			// Special handling for GPE (Geopolitical Entity)
			if strings.Contains(upperVal, "GPE") {
				label = strings.Replace(upperVal, "GPE", "LOCATION", 1)
			} else {
				label = strings.Replace(upperVal, "LOC", "LOCATION", 1)
			}
		}
		
		labels[idx] = label
	}

	return labels, nil
}

// Close releases the ONNX session resources
func (p *ONNXProvider) Close() {
	if p.Session != nil {
		p.Session.Destroy()
	}
	if p.inputIdsTensor != nil {
		p.inputIdsTensor.Destroy()
	}
	if p.attentionMaskTensor != nil {
		p.attentionMaskTensor.Destroy()
	}
	if p.tokenTypeIdsTensor != nil {
		p.tokenTypeIdsTensor.Destroy()
	}
	if p.outputTensor != nil {
		p.outputTensor.Destroy()
	}
}
