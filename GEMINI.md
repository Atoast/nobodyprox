# Project Context: NobodyProx

## 1. Engineering Standards & Architecture
- **Single-Pass Redaction**: The `filter.Engine` uses a collection-first, single-pass replacement logic (iterating backwards from end-to-start) to prevent recursive redaction of placeholders.
- **Request Correlation**: Every request must be assigned a unique `reqID` in `Proxy.ServeHTTP` and passed through all filtering layers to ensure logs are correctly correlated.
- **NER-Aware Rules**: PII redaction for NER findings is granular. Entities are only redacted if a rule with a matching `entity_type` exists in `config.yaml`.
- **CGO Dependency**: This project **requires CGO** for the ONNX Runtime. Development and building must occur in an environment with a C compiler (like MinGW-w64 on Windows).

## 2. Bootstrapping & Setup
- **One-Command Setup**: Use `go run cmd/main.go setup` to automate the download of ONNX binaries, models, and the installation of the Root CA into the system trust store.
- **Automatic DLL Discovery**: `onnxruntime.dll` is expected in the project root. The `BootstrapONNX` logic handles downloading this from the global `onnx_runtime_url` in `config.yaml`.
- **Model Discovery**: The proxy automatically parses Hugging Face `config.json` files to discover and normalize NER labels.

## 3. UI & Interaction
- **TUI-First**: The application launches the interactive dashboard by default. Standard CLI logging is available via the `--no-tui` flag.
- **Rule Builder**: Press `[tab]` in the TUI to access the interactive Rule Builder for live testing of PII patterns without making network requests.

## 4. Scandinavian Focus
- Priority is given to high-accuracy Danish and Scandinavian NER support (currently using the `mmbert-scandi` configuration).
