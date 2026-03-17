package filter

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nobodyprox/nobodyprox/pkg/config"
)

// BootstrapAll prepares all models defined in the configuration
func BootstrapAll(cfg *config.Config) error {
	log.Println("[Bootstrap] Starting full environment setup...")
	for name, m := range cfg.ONNXModels {
		log.Printf("[Bootstrap] Preparing model: %s", name)
		err := BootstrapONNX(m.ModelPath, m.VocabPath, m.ConfigPath, cfg.ONNXRuntimeURL, m.ModelDownloadURL, m.VocabDownloadURL, m.ConfigDownloadURL)
		if err != nil {
			return fmt.Errorf("failed to bootstrap model %s: %v", name, err)
		}
	}
	return nil
}

// BootstrapONNX checks for and downloads missing ONNX resources
func BootstrapONNX(modelPath, vocabPath, configPath, onnxURL, modelURL, vocabURL, configURL string) error {
	// 1. Check/Download onnxruntime shared library
	libName := getSharedLibName()
	if _, err := os.Stat(libName); os.IsNotExist(err) {
		log.Printf("[Bootstrap] %s missing. Starting download...", libName)
		if err := downloadAndExtractDLL(onnxURL, libName); err != nil {
			return fmt.Errorf("failed to bootstrap %s: %v", libName, err)
		}
		log.Printf("[Bootstrap] %s installed successfully.", libName)
	}

	// 2. Ensure models directory exists
	modelsDir := filepath.Dir(modelPath)
	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(modelsDir, 0755); err != nil {
			return fmt.Errorf("failed to create models directory: %v", err)
		}
	}

	// 3. Check/Download Model
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		log.Printf("[Bootstrap] Model missing at %s. Starting download...", modelPath)
		if err := downloadFile(modelURL, modelPath); err != nil {
			return fmt.Errorf("failed to bootstrap model: %v", err)
		}
		log.Println("[Bootstrap] Model downloaded successfully.")
	}

	// 4. Check/Download Vocab/Tokenizer
	if _, err := os.Stat(vocabPath); os.IsNotExist(err) {
		label := "Vocab"
		if strings.HasSuffix(vocabPath, ".json") {
			label = "Tokenizer"
		}
		log.Printf("[Bootstrap] %s missing at %s. Starting download...", label, vocabPath)
		if err := downloadFile(vocabURL, vocabPath); err != nil {
			return fmt.Errorf("failed to bootstrap %s: %v", label, err)
		}
		log.Printf("[Bootstrap] %s downloaded successfully.", label)
	}

	// 5. Check/Download Config (Optional)
	if configPath != "" && configURL != "" {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			log.Printf("[Bootstrap] Config missing at %s. Starting download...", configPath)
			if err := downloadFile(configURL, configPath); err != nil {
				return fmt.Errorf("failed to bootstrap config: %v", err)
			}
			log.Println("[Bootstrap] Config downloaded successfully.")
		}
	}

	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func downloadAndExtractDLL(url, dest string) error {
	tmpZip := "onnxruntime_tmp.zip"
	if err := downloadFile(url, tmpZip); err != nil {
		return err
	}
	defer os.Remove(tmpZip)

	r, err := zip.OpenReader(tmpZip)
	if err != nil {
		return err
	}
	defer r.Close()

	// Find the shared library file in the zip
	// Windows: onnxruntime.dll
	// Linux: libonnxruntime.so
	// MacOS: libonnxruntime.dylib
	var found bool
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/"+dest) || f.Name == dest {
			found = true
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return err
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("%s not found in zip archive", dest)
	}

	return nil
}
