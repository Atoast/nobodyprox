package config

import (
	"os"

	"github.com/nobodyprox/nobodyprox/pkg/filter"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ProxyPort   int           `yaml:"proxy_port"`
	NERProvider string        `yaml:"ner_provider"`
	ModelPath   string        `yaml:"model_path"`
	VocabPath   string        `yaml:"vocab_path"`
	Rules       []filter.Rule `yaml:"rules"`
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

	return &cfg, nil
}

func createDefaultConfig(path string) (*Config, error) {
	cfg := &Config{
		ProxyPort:   8080,
		NERProvider: "prose",
		Rules: []filter.Rule{
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
		},
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
