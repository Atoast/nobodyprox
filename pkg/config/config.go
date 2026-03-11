package config

import (
	"os"

	"github.com/nobodyprox/nobodyprox/pkg/filter"
	"gopkg.in/yaml.v3"
)

type ONNXModelConfig struct {
	ModelPath        string         `yaml:"model_path"`
	VocabPath        string         `yaml:"vocab_path"`
	ModelDownloadURL string         `yaml:"model_download_url"`
	VocabDownloadURL string         `yaml:"vocab_download_url"`
	Labels           map[int]string `yaml:"labels"`
}

type Config struct {
	ProxyPort      int                          `yaml:"proxy_port"`
	NERProvider    string                       `yaml:"ner_provider"`
	ActiveModel    string                       `yaml:"active_model"`
	WatchMode      bool                         `yaml:"watch_mode"`
	ONNXRuntimeURL string                       `yaml:"onnx_runtime_url"`
	ONNXModels     map[string]ONNXModelConfig   `yaml:"onnx_models"`
	Rules          []filter.Rule                `yaml:"rules"`
}

// LoadConfig reads the configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return createDefaultConfig(path)
		}
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
	        return nil, err
	}

	// Apply defaults for missing runtime URL

	if cfg.ONNXRuntimeURL == "" {
		cfg.ONNXRuntimeURL = "https://github.com/microsoft/onnxruntime/releases/download/v1.24.1/onnxruntime-win-x64-1.24.1.zip"
	}

	return &cfg, nil
}

func createDefaultConfig(path string) (*Config, error) {
	content := `# NobodyProx Configuration

# The port the proxy will listen on
proxy_port: 8080

# The NER (Named Entity Recognition) provider to use.
# Options: 
#   - "prose": Fast, pure-Go provider (default).
#   - "onnx": High-accuracy ML provider (requires onnxruntime.dll).
ner_provider: prose

# The active ONNX model to use from the onnx_models map below.
active_model: bert-multilingual

# When true, the proxy logs sensitive data found but does NOT redact it.
# Can be overridden by the --watch command line flag.
watch_mode: false

# URL to download the ONNX Runtime DLL if missing (Windows x64).
onnx_runtime_url: https://github.com/microsoft/onnxruntime/releases/download/v1.17.1/onnxruntime-win-x64-1.17.1.zip

# Map of available ONNX models and their download locations.
onnx_models:
    bert-multilingual:
        model_path: models/model.onnx
        vocab_path: models/vocab.txt
        model_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/onnx/model.onnx
        vocab_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/vocab.txt
    bert-multilingual-quantized:
        model_path: models/model_quantized.onnx
        vocab_path: models/vocab.txt
        model_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/onnx/model_quantized.onnx
        vocab_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/vocab.txt

# Custom regex rules for PII detection.
# Actions:
#   - REDACT: Replaces matches with [REDACTED: RuleName] (default).
#   - PSEUDONYMIZE: Replaces matches with a consistent hash, e.g., [RuleName_a1b2c3d4].
rules:
    - name: OPENAI_KEY
      pattern: sk-[a-zA-Z0-9]{48}
      action: REDACT
    - name: DANISH_CPR
      pattern: \b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-\d{4}\b
      action: REDACT
    - name: EMAIL
      pattern: \b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b
      action: REDACT
`
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return nil, err
	}

	return LoadConfig(path)
}
