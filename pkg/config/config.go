package config

import (
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Rule represents a single redaction or pseudonymization rule
type Rule struct {
	Name        string         `yaml:"name"`
	Pattern     string         `yaml:"pattern"`
	EntityType  string         `yaml:"entity_type"`
	Replacement string         `yaml:"replacement"`
	Action      string         `yaml:"action"`
	Regex       *regexp.Regexp `yaml:"-"`
}

type ONNXModelConfig struct {
	ModelPath         string         `yaml:"model_path"`
	VocabPath         string         `yaml:"vocab_path"`
	ConfigPath        string         `yaml:"config_path"`
	ModelDownloadURL  string         `yaml:"model_download_url"`
	VocabDownloadURL  string         `yaml:"vocab_download_url"`
	ConfigDownloadURL string         `yaml:"config_download_url"`
	Labels            map[int]string `yaml:"labels"`
}

type Config struct {
	ProxyPort       int                        `yaml:"proxy_port"`
	NERProvider     string                     `yaml:"ner_provider"`
	ActiveModel     string                     `yaml:"active_model"`
	WatchMode       bool                       `yaml:"watch_mode"`
	RedactResponses bool                       `yaml:"redact_responses"`
	FilterDomains   []string                   `yaml:"filter_domains"`
	ONNXRuntimeURL  string                     `yaml:"onnx_runtime_url"`
	ONNXModels      map[string]ONNXModelConfig `yaml:"onnx_models"`
	Rules           []Rule                     `yaml:"rules"`
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

	// Apply defaults
	if cfg.ONNXRuntimeURL == "" {
		cfg.ONNXRuntimeURL = "https://github.com/microsoft/onnxruntime/releases/download/v1.24.1/onnxruntime-win-x64-1.24.1.zip"
	}

	return &cfg, nil
}

// SaveConfig writes the configuration back to the YAML file
func SaveConfig(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(4)
	return encoder.Encode(cfg)
}

func createDefaultConfig(path string) (*Config, error) {
	content := `# NobodyProx Configuration

# The port the proxy will listen on
proxy_port: 8080

# The NER (Named Entity Recognition) provider to use.
# Options: 
#   - "prose": Fast, pure-Go provider (default).
#   - "onnx": High-accuracy ML provider (requires onnxruntime.dll).
ner_provider: onnx

# The active ONNX model to use from the onnx_models map below.
active_model: mmbert-scandi

# When true, the proxy logs sensitive data found but does NOT redact it.
# Can be overridden by the --watch command line flag.
watch_mode: false

# When true, the proxy redacts both outgoing requests and incoming responses.
# When false, only outgoing requests are processed (saves performance and noise).
redact_responses: true

# List of domains to filter. If empty, ALL domains will be filtered.
# Example: ["openai.com", "anthropic.com", "httpbin.org"]
filter_domains: []

# URL to download the ONNX Runtime DLL if missing (Windows x64).
onnx_runtime_url: https://github.com/microsoft/onnxruntime/releases/download/v1.24.1/onnxruntime-win-x64-1.24.1.zip

# Map of available ONNX models and their download locations.
onnx_models:
    bert-multilingual:
        model_path: models/bert-multilingual/model.onnx
        vocab_path: models/bert-multilingual/vocab.txt
        config_path: models/bert-multilingual/config.json
        model_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/onnx/model.onnx
        vocab_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/vocab.txt
        config_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/config.json
    bert-multilingual-quantized:
        model_path: models/bert-multilingual-quantized/model.onnx
        vocab_path: models/bert-multilingual-quantized/vocab.txt
        config_path: models/bert-multilingual-quantized/config.json
        model_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/onnx/model_quantized.onnx
        vocab_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/vocab.txt
        config_download_url: https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-hrl/resolve/main/config.json
    mmbert-scandi:
        model_path: models/mmbert-scandi/model.onnx
        vocab_path: models/mmbert-scandi/tokenizer.json
        config_path: models/mmbert-scandi/config.json
        # Note: You still need to manually convert the Scandi model to ONNX for now, 
        # or provide a direct download URL if available.
        config_download_url: https://huggingface.co/MediaCatch/mmBERT-base-scandi-ner/resolve/main/config.json

# Custom regex and NER rules for PII detection.
# Rules can be based on a regex "pattern" or an NER "entity_type".
# Actions:
#   - REDACT: Replaces matches with [REDACTED: RuleName/EntityType] (default).
#   - PSEUDONYMIZE: Replaces matches with a consistent hash, e.g., [RuleName_a1b2c3d4].
rules:
    # Regex Rules (Fast Path)
    - name: OPENAI_KEY
      pattern: sk-[a-zA-Z0-9]{48}
      action: REDACT
    - name: DANISH_CPR
      pattern: \b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-\d{4}\b
      action: REDACT
    - name: EMAIL
      pattern: \b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b
      action: REDACT

    # NER Rules (Deep Path - requires ONNX or Prose)
    - name: PERSON_DETECTION
      entity_type: PERSON
      action: PSEUDONYMIZE
    - name: ORG_DETECTION
      entity_type: ORGANIZATION
      action: REDACT
    - name: LOC_DETECTION
      entity_type: LOCATION
      action: REDACT
    - name: MISC_DETECTION
      entity_type: MISC
      action: REDACT
`
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return nil, err
	}

	return LoadConfig(path)
}
