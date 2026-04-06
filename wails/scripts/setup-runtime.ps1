$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "[logos-setup] $Message"
}

function Get-Setting {
    param(
        [string]$Name,
        [string]$Fallback
    )
    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Fallback
    }
    return $value
}

$DataDir = Get-Setting "LOGOS_DATA_DIR" (Join-Path $env:LOCALAPPDATA "Logos AI")
$BinDir = Get-Setting "LOGOS_BIN_DIR" (Join-Path $DataDir "bin")
$VenvDir = Get-Setting "LOGOS_VENV_DIR" (Join-Path $DataDir "venv")
$PiperDir = Get-Setting "LOGOS_PIPER_DIR" (Join-Path $DataDir "piper")
$KokoroDir = Get-Setting "LOGOS_KOKORO_DIR" (Join-Path $DataDir "kokoro")
$PythonPointer = Get-Setting "LOGOS_PYTHON_POINTER" (Join-Path $DataDir "python_interp")
$ChatModel = Get-Setting "LOGOS_OLLAMA_MODEL" "llama3.2:3b"
$EmbedModel = Get-Setting "LOGOS_OLLAMA_EMBED_MODEL" "embeddinggemma"
$KokoroScriptSource = Get-Setting "LOGOS_KOKORO_SCRIPT_SOURCE" (Join-Path $DataDir "kokoro_speak.py")
$KokoroModelUrl = Get-Setting "LOGOS_KOKORO_MODEL_URL" "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.onnx"
$KokoroVoicesUrl = Get-Setting "LOGOS_KOKORO_VOICES_URL" "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/voices-v1.0.bin"

function Test-PythonVersion {
    param(
        [string]$Command,
        [string[]]$Arguments
    )

    & $Command @($Arguments + @("-c", "import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)")) | Out-Null
    return $LASTEXITCODE -eq 0
}

function Get-PythonCandidate {
    $candidates = @(
        @{ Command = "py"; Arguments = @("-3.12") },
        @{ Command = "py"; Arguments = @("-3.11") },
        @{ Command = "py"; Arguments = @("-3.10") },
        @{ Command = "python"; Arguments = @() },
        @{ Command = "python3"; Arguments = @() }
    )

    foreach ($candidate in $candidates) {
        if (-not (Get-Command $candidate.Command -ErrorAction SilentlyContinue)) {
            continue
        }
        if (Test-PythonVersion -Command $candidate.Command -Arguments $candidate.Arguments) {
            return $candidate
        }
    }

    return $null
}

function Ensure-Python {
    $candidate = Get-PythonCandidate
    if ($null -ne $candidate) {
        return $candidate
    }

    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        throw "Python 3.10+ was not found and winget is unavailable. Install Python from https://www.python.org/downloads/ and rerun setup."
    }

    Write-Step "Installing Python 3.11 with winget..."
    winget install --id Python.Python.3.11 --silent --accept-source-agreements --accept-package-agreements

    $candidate = Get-PythonCandidate
    if ($null -eq $candidate) {
        throw "Python installation completed, but Python 3.10+ is still unavailable on PATH."
    }

    return $candidate
}

function Download-FileIfMissing {
    param(
        [string]$Path,
        [string]$Url
    )

    if (Test-Path $Path) {
        Write-Step ("Already present: {0}" -f $Path)
        return
    }

    Write-Step ("Downloading {0}" -f $Url)
    Invoke-WebRequest -Uri $Url -OutFile $Path
}

function Ensure-Ollama {
    if (Get-Command ollama -ErrorAction SilentlyContinue) {
        return
    }

    Write-Step "Installing Ollama from the official installer..."
    Invoke-RestMethod https://ollama.com/install.ps1 | Invoke-Expression
}

function Start-OllamaIfNeeded {
    if (-not (Get-Command ollama -ErrorAction SilentlyContinue)) {
        throw "Ollama is not available on PATH."
    }

    & ollama list | Out-Null
    if ($LASTEXITCODE -eq 0) {
        return
    }

    Write-Step "Starting Ollama in the background..."
    Start-Process -FilePath "ollama" -ArgumentList "serve" -WindowStyle Hidden

    for ($i = 0; $i -lt 30; $i++) {
        Start-Sleep -Seconds 1
        & ollama list | Out-Null
        if ($LASTEXITCODE -eq 0) {
            return
        }
    }

    throw "Ollama did not become ready in time."
}

function Get-InstalledOllamaModels {
    $output = & ollama list 2>$null
    if ($LASTEXITCODE -ne 0) {
        return @()
    }

    $models = @()
    foreach ($line in ($output -split "`r?`n" | Select-Object -Skip 1)) {
        $parts = ($line.Trim() -split "\s+") | Where-Object { $_ -ne "" }
        if ($parts.Count -gt 0) {
            $models += $parts[0]
        }
    }
    return $models
}

function Pull-OllamaModelIfMissing {
    param([string]$Model)

    if ((Get-InstalledOllamaModels) -contains $Model) {
        Write-Step ("Model already present: {0}" -f $Model)
        return
    }

    Write-Step ("Pulling Ollama model: {0}" -f $Model)
    & ollama pull $Model
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to pull Ollama model: {0}" -f $Model)
    }
}

Write-Step "Preparing local AI and voice runtime..."
New-Item -ItemType Directory -Force -Path $DataDir, $BinDir, $PiperDir, $KokoroDir | Out-Null

$python = Ensure-Python
Write-Step ("Using Python command: {0} {1}" -f $python.Command, ($python.Arguments -join " "))

if (-not (Test-Path (Join-Path $VenvDir "Scripts\python.exe"))) {
    Write-Step ("Creating virtual environment at {0}" -f $VenvDir)
    & $python.Command @($python.Arguments + @("-m", "venv", $VenvDir))
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create the virtual environment."
    }
} else {
    Write-Step "Virtual environment already exists"
}

$Py = Join-Path $VenvDir "Scripts\python.exe"
$Pip = Join-Path $VenvDir "Scripts\pip.exe"
$env:PATH = "$BinDir;$($VenvDir)\Scripts;$env:PATH"

Write-Step "Upgrading pip"
& $Py -m pip install --upgrade pip

Write-Step "Installing Kokoro and Piper dependencies"
& $Pip install --upgrade `
    kokoro-onnx `
    onnxruntime `
    numpy `
    soundfile `
    piper-tts

Download-FileIfMissing -Path (Join-Path $KokoroDir "kokoro-v1.0.onnx") -Url $KokoroModelUrl
Download-FileIfMissing -Path (Join-Path $KokoroDir "voices-v1.0.bin") -Url $KokoroVoicesUrl

if (-not (Test-Path $KokoroScriptSource)) {
    throw ("Expected Kokoro wrapper source at {0}" -f $KokoroScriptSource)
}

Copy-Item -Force $KokoroScriptSource (Join-Path $BinDir "kokoro-speak.py")

$kokoroCmd = @"
@echo off
set "LOGOS_KOKORO_DIR=$KokoroDir"
"$Py" "$BinDir\kokoro-speak.py" %*
"@
Set-Content -Path (Join-Path $BinDir "kokoro-speak.cmd") -Value $kokoroCmd -Encoding ASCII

$piperCommand = Get-Command piper -ErrorAction SilentlyContinue
if ($null -eq $piperCommand) {
    $piperCmd = @"
@echo off
"$Py" -m piper %*
"@
    Set-Content -Path (Join-Path $BinDir "piper.cmd") -Value $piperCmd -Encoding ASCII
}

Set-Content -Path $PythonPointer -Value $Py -Encoding ASCII
Write-Step ("Saved interpreter pointer to {0}" -f $PythonPointer)

try {
    Ensure-Ollama
    Start-OllamaIfNeeded
    Pull-OllamaModelIfMissing -Model $ChatModel
    Pull-OllamaModelIfMissing -Model $EmbedModel
} catch {
    Write-Step $_.Exception.Message
    Write-Step "Install or start Ollama from https://ollama.com/download and rerun setup to finish local AI."
}

Write-Step "Setup complete."
Write-Step "Restart Logos AI if the voice picker or AI tools do not refresh automatically."
