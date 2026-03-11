#!/usr/bin/env python3
import sys
import subprocess
import os

def main():
    if len(sys.argv) < 2:
        print("Usage: python convert_model.py <model_name_or_path> [output_dir]")
        print("Example: python convert_model.py MediaCatch/mmBERT-base-scandi-ner ./models/mmbert-scandi")
        sys.exit(1)

    model_name = sys.argv[1]
    output_dir = sys.argv[2] if len(sys.argv) > 2 else f"./models/{model_name.split('/')[-1]}"

    print(f"[*] Converting model '{model_name}' to ONNX format...")
    print(f"[*] Output directory: {output_dir}")

    # Ensure output directory exists
    os.makedirs(output_dir, exist_ok=True)

    # Command: optimum-cli export onnx --model <model_name> --task token-classification <output_dir>
    cmd = [
        "optimum-cli", "export", "onnx",
        "--model", model_name,
        "--task", "token-classification",
        output_dir
    ]

    print(f"[*] Running: {' '.join(cmd)}")
    try:
        subprocess.run(cmd, check=True)
        print("[+] Conversion successful!")
        
        # Helper info for config.yaml
        print("\n[!] To use this model in NobodyProx, update your config.yaml:")
        print(f"active_model: {model_name.split('/')[-1]}")
        print("onnx_models:")
        print(f"    {model_name.split('/')[-1]}:")
        print(f"        model_path: {os.path.join(output_dir, 'model.onnx')}")
        print(f"        vocab_path: {os.path.join(output_dir, 'tokenizer.json')}")
        print("        labels:")
        print("            # You'll need to map the labels from config.json here")
        print("            # Example for scandi-ner:")
        print("            1: PERSON")
        print("            2: PERSON")
        print("            ...")
        
    except subprocess.CalledProcessError as e:
        print(f"[-] Error during conversion: {e}")
        print("[!] Ensure you have optimum installed: pip install optimum[onnxruntime]")
        sys.exit(1)

if __name__ == "__main__":
    main()
