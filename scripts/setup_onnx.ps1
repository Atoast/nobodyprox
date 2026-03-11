# Setup Script for NobodyProx ONNX Support
# This script downloads the required binaries and models for the ONNX NER Provider.

$ProjectRoot = Get-Item "."
$BinPath = "$ProjectRoot\onnxruntime.dll"
$ModelPath = "$ProjectRoot\models\distilbert-multilingual-ner.onnx"
$VocabPath = "$ProjectRoot\models\vocab.txt"

# 1. Ensure models directory exists
if (-not (Test-Path "$ProjectRoot\models")) {
    New-Item -ItemType Directory -Path "$ProjectRoot\models" | Out-Null
    Write-Host "[*] Created models directory." -ForegroundColor Cyan
}

# 2. Download ONNX Runtime DLL (Windows x64)
if (-not (Test-Path $BinPath)) {
    Write-Host "[*] Downloading onnxruntime.dll..." -ForegroundColor Yellow
    $OnnxZip = "$env:TEMP\onnxruntime.zip"
    $Url = "https://github.com/microsoft/onnxruntime/releases/download/v1.17.1/onnxruntime-win-x64-1.17.1.zip"
    
    # Use curl.exe for better redirect handling
    curl.exe -L -o $OnnxZip $Url
    
    if (Test-Path $OnnxZip) {
        Expand-Archive -Path $OnnxZip -DestinationPath "$env:TEMP\onnx_extract" -Force
        Copy-Item "$env:TEMP\onnx_extract\onnxruntime-win-x64-1.17.1\lib\onnxruntime.dll" -Destination $BinPath
        Remove-Item $OnnxZip
        Remove-Item "$env:TEMP\onnx_extract" -Recurse
        Write-Host "[+] Successfully installed onnxruntime.dll." -ForegroundColor Green
    } else {
        Write-Host "[!] Failed to download onnxruntime.dll." -ForegroundColor Red
    }
} else {
    Write-Host "[+] onnxruntime.dll already exists." -ForegroundColor Gray
}

# 3. Download Pre-trained Multilingual NER Model (ONNX)
if (-not (Test-Path $ModelPath)) {
    Write-Host "[*] Downloading NER Model (distilbert-multilingual)..." -ForegroundColor Yellow
    $ModelUrl = "https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-main/resolve/main/onnx/model.onnx"
    curl.exe -L -o $ModelPath $ModelUrl
    if (Test-Path $ModelPath) {
        Write-Host "[+] Successfully downloaded NER model." -ForegroundColor Green
    } else {
        Write-Host "[!] Failed to download NER model." -ForegroundColor Red
    }
} else {
    Write-Host "[+] NER model already exists." -ForegroundColor Gray
}

# 4. Download Vocab File
if (-not (Test-Path $VocabPath)) {
    Write-Host "[*] Downloading vocab.txt..." -ForegroundColor Yellow
    $VocabUrl = "https://huggingface.co/Xenova/bert-base-multilingual-cased-ner-main/resolve/main/vocab.txt"
    curl.exe -L -o $VocabPath $VocabUrl
    if (Test-Path $VocabPath) {
        Write-Host "[+] Successfully downloaded vocab file." -ForegroundColor Green
    } else {
        Write-Host "[!] Failed to download vocab file." -ForegroundColor Red
    }
} else {
    Write-Host "[+] Vocab file already exists." -ForegroundColor Gray
}

Write-Host "`n[DONE] Setup complete! You can now configure NobodyProx to use the ONNX provider." -ForegroundColor Green
Write-Host "Update your config.yaml with:" -ForegroundColor Cyan
Write-Host "  ner_provider: onnx"
Write-Host "  model_path: models/distilbert-multilingual-ner.onnx"
Write-Host "  vocab_path: models/vocab.txt"
