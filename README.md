# NobodyProx: Privacy Proxy for AI Tools

NobodyProx is a fast, transparent local proxy designed to intercept and filter sensitive data from outgoing traffic. It is specifically optimized to work alongside AI tools (like coding assistants, CLI agents, and chatbots) without introducing noticeable latency.

## Key Features
- **Transparency:** Seamlessly intercepts HTTPS traffic via a local Root CA (MITM).
- **Hybrid Filtering:** Uses high-performance Regex for structured data and optional NER (Named Entity Recognition) for unstructured data.
- **Pseudonymization:** Consistently replaces sensitive entities (e.g., names) with synthetic placeholders to maintain AI context.
- **Multilingual Support:** Special focus on English and Danish language support.

---

## 🛠 Setup & Installation

### 1. Prerequisites
> **Note:** This project requires **CGO** for ONNX Runtime. You **must** have a C compiler installed (like MinGW) to build the solution.

- **Go 1.21+**
- **C Compiler:** [MinGW-w64](https://www.mingw-w64.org/) or [w64devkit](https://github.com/skeeto/w64devkit) (Required for CGO).
- **ONNX Runtime Binary:** Download `onnxruntime.dll` (Windows) from the [Official ONNX Releases](https://github.com/microsoft/onnxruntime/releases) and place it in the project root.

### 2. Building the Project
NobodyProx requires **CGO** to be enabled for the ONNX runtime.

#### Windows (PowerShell):
```powershell
# 1. Add your C compiler to the PATH (if not already there)
$env:PATH += ";C:\path\to\w64devkit\bin"

# 2. Enable CGO and build
$env:CGO_ENABLED="1"
go build -o nobodyprox.exe ./cmd/main.go
```

### 3. Trusting the Root CA
Upon first run, NobodyProx generates a local Root CA in `./certs/ca.crt`. 
- To intercept HTTPS traffic without security warnings, you must **install and trust** this certificate in your Operating System or specific toolchain (e.g., `git config --global http.sslCAInfo certs/ca.crt`).

---

## ⚙️ Configuration
The first time you run the proxy, it will generate a default `config.yaml` file:

- `proxy_port`: The port the proxy listens on (default: `8080`).
- `ner_provider`: Choose between `prose` (Fast, Pure Go) or `onnx` (High-Accuracy, ML).
- `model_path`: Path to your `.onnx` model file (required for ONNX provider).
- `rules`: A list of Regex-based redaction/pseudonymization rules.

### Example Rule:
```yaml
rules:
  - name: DANISH_CPR
    pattern: '\b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-\d{4}\b'
    action: REDACT # or PSEUDONYMIZE
```

---

## 🧪 Testing
Run the automated test suite to verify the filtering logic:
```bash
go test -v ./tests/...
```

---

## ⚖️ License
MIT License - See [LICENSE](LICENSE) for more details.
