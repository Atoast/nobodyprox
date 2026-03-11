package filter

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const (
	onnxDownloadURL  = "https://github.com/microsoft/onnxruntime/releases/download/v1.17.1/onnxruntime-win-x64-1.17.1.zip"
	modelDownloadURL = "https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-main/resolve/main/onnx/model.onnx"
	vocabDownloadURL = "https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-main/resolve/main/vocab.txt"
)

// BootstrapONNX checks for and downloads missing ONNX resources
func BootstrapONNX(modelPath, vocabPath string) error {
	// 1. Check/Download onnxruntime.dll
	dllPath := "onnxruntime.dll"
	if _, err := os.Stat(dllPath); os.IsNotExist(err) {
		log.Println("[Bootstrap] onnxruntime.dll missing. Starting download...")
		if err := downloadAndExtractDLL(onnxDownloadURL, dllPath); err != nil {
			return fmt.Errorf("failed to bootstrap onnxruntime.dll: %v", err)
		}
		log.Println("[Bootstrap] onnxruntime.dll installed successfully.")
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
		if err := downloadFile(modelDownloadURL, modelPath); err != nil {
			return fmt.Errorf("failed to bootstrap model: %v", err)
		}
		log.Println("[Bootstrap] Model downloaded successfully.")
	}

	// 4. Check/Download Vocab
	if _, err := os.Stat(vocabPath); os.IsNotExist(err) {
		log.Printf("[Bootstrap] Vocab missing at %s. Starting download...", vocabPath)
		if err := downloadFile(vocabDownloadURL, vocabPath); err != nil {
			return fmt.Errorf("failed to bootstrap vocab: %v", err)
		}
		log.Println("[Bootstrap] Vocab downloaded successfully.")
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

	// The DLL is located inside: onnxruntime-win-x64-1.17.1/lib/onnxruntime.dll
	targetInZip := "onnxruntime-win-x64-1.17.1/lib/onnxruntime.dll"
	var found bool
	for _, f := range r.File {
		if f.Name == targetInZip {
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
		return fmt.Errorf("onnxruntime.dll not found in zip archive")
	}

	return nil
}
