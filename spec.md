# Privacy Proxy Specification

## 1. Overview
A fast, transparent proxy designed to intercept and filter sensitive data from outgoing traffic. It is specifically optimized to work seamlessly alongside AI tools (like coding assistants, CLI agents, and chatbots) without introducing noticeable latency. 

## 2. Goals
- **Transparency:** Act as a seamless proxy without breaking existing AI tool workflows or requiring complex client-side configuration.
- **Performance:** Maintain minimal latency overhead to ensure smooth, real-time interactions with AI services.
- **Privacy:** Accurately identify and redact or anonymize sensitive information (e.g., PII, credentials, internal secrets) before it leaves the local or corporate environment.

## 3. Architecture Structure
- **Core Technology:** Go (Golang) for high concurrency, low latency, and a strong networking ecosystem.
- **Interception Strategy:** Explicit Forward Proxy. The proxy will run locally (e.g., `localhost:8080`), and users will configure their OS or specific AI tools to route traffic through it.
- **TLS/HTTPS Interception:** To inspect payloads, the proxy will utilize a local Root CA to decrypt HTTPS traffic (MITM), inspect the content, and re-encrypt it before forwarding.
- **Filtering Engine:** High-performance hybrid engine.
  - **Fast Path (Default):** Regex and string-matching for structured data (API keys, SSNs).
  - **Deep Path (Optional):** Lightweight Named-entity recognition (NER) for unstructured PII (Names, Addresses). Potential implementation paths:
    - **Option A (Pure Go):** `Prose (v3)` for zero-dependency, high-speed basic NER.
    - **Option B (AI Runtime):** `Go-ONNX` to run lightweight transformer models (e.g., DistilBERT-multilingual) for better Danish/English accuracy without a Python dependency.
    - **Option C (High Accuracy):** `Go-spaCy` or a `Python Sidecar (DaCy)` for state-of-the-art Danish NLP (best catch-rate, highest latency).
  - **Confidence Control:** Configurable thresholds (0.0 to 1.0) for each entity type to balance between "Recall" (catch everything) and "Precision" (minimize false positives).

## 4. Filtering Capabilities
- **Pre-defined Rules:** Common credentials (AWS, GitHub, Stripe keys, etc.), standard PII (Credit Cards, SSNs).
- **Named-entity recognition (NER):** Optional context-aware detection for personal names, locations, and organizations. Special focus on **Danish (DaCy/DaNE)** and **English** support.
  - **Configurable Thresholds:** Users can set minimum confidence levels per entity type in the configuration.
- **Custom Rules:** User-defined Regex or heuristics to match proprietary internal data patterns.
- **Action:** Replace matched sensitive data with standardized placeholders (e.g., `[REDACTED: API_KEY]`).

## 5. Configuration & Management
- **Initial Phase (v1):** 
  - **Configuration File:** All filtering rules, allowlists, and denylists will be managed via a clean `config.yaml` or `.env` file.
  - **CLI Tool:** A command-line interface to start, stop, and manage the proxy, as well as view real-time logs and statistics.
- **Future Phase (v2):**
  - **Local Web UI:** A local web server dashboard for easier management of rules, visualizing intercepted traffic, and auditing logs.

## 6. Development Phases
- **Phase 1: Core Proxy & HTTPS MITM** - Establish the Go-based forward proxy and handle TLS certificate generation and decryption.
- **Phase 2: Filtering Engine & Configuration** - Implement the regex/pattern-matching engine, integrate the YAML configuration loader, and build the initial predefined rulesets. 
- **Phase 2.1: NER Integration & Language Support** - Evaluate and integrate a lightweight NER model (Option A, B, or C) as an optional layer. Prioritize high-performance Danish and English support (e.g., using ONNX or DaCy).
- **Phase 3: CLI Polish & Distribution** - Finalize the CLI commands, logging formatting, and create release binaries for major operating systems.

## 7. Data Dictionary (Initial Focus)
To ensure high accuracy, we will maintain a "Data Dictionary" defining the patterns and characteristics of each sensitive data type:
- **`SECRET_API_KEY`**: High-entropy strings, often with known prefixes (e.g., `sk_live_...`, `AIza...`, `ghp_...`).
- **`PII_CPR_DK`**: Danish Civil Registration Numbers (format: `DDMMYY-XXXX`).
- **`PII_CREDIT_CARD`**: 16-digit numbers matching Luhn algorithm validation.
- **`PII_EMAIL`**: Standard email address patterns.
- **`PII_NAME`**: Identified via NER (Deep Path) or context-aware heuristics (e.g., following "My name is...").

## 8. Testing & Validation Strategy
A dedicated testing suite will ensure the proxy is both effective (high catch rate) and usable (low false positives):
- **Golden Dataset:** A JSON/YAML file containing "Input" (sensitive text) and "Expected Output" (redacted text) for every data type.
- **False Positive Suite:** A collection of "Safe" strings that look like secrets but are legitimate (e.g., hashes in lockfiles, UUIDs) to prevent over-redaction.
- **Performance Benchmarks:** Automated tests to measure the latency overhead (ms) per kilobyte of traffic to ensure it remains "AI-tool friendly."
- **Regression Testing:** Every new rule or NER model update must pass the full Golden Dataset and False Positive Suite.