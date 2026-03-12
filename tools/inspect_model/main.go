package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	ort "github.com/yalue/onnxruntime_go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run tools/inspect_model.go <path_to_model.onnx>")
		return
	}

	modelPath := os.Args[1]

	// Initialize environment
	libPath := "onnxruntime.dll"
	absPath, _ := filepath.Abs(libPath)
	ort.SetSharedLibraryPath(absPath)
	err := ort.InitializeEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	defer ort.DestroyEnvironment()

	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		log.Fatalf("Failed to get model info: %v", err)
	}

	fmt.Printf("\n--- Model: %s ---\n", modelPath)
	fmt.Println("\nInputs:")
	for i, input := range inputs {
		fmt.Printf("  [%d] Name: %s, Shape: %v\n", i, input.Name, input.Dimensions)
	}

	fmt.Println("\nOutputs:")
	for i, output := range outputs {
		fmt.Printf("  [%d] Name: %s, Shape: %v\n", i, output.Name, output.Dimensions)
	}
}
