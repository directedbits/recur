# Setup script for the recur project.
# Installs development dependencies and prepares the repository for building.
# Compatible with PowerShell 5.1+.
#
# Usage: .\setup.ps1 [-DryRun]

param(
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$MinGoVersion = [Version]"1.25"

function Write-Log {
    param([string]$Message)
    Write-Host "[setup] $Message"
}

function Write-Error-And-Exit {
    param([string]$Message)
    Write-Host "[setup] ERROR: $Message" -ForegroundColor Red
    exit 1
}

function Test-Command {
    param([string]$Name)
    $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Invoke-Step {
    param([string]$Description, [scriptblock]$Action)
    if ($DryRun) {
        Write-Log "(dry-run) $Description"
    } else {
        & $Action
    }
}

# --- Check Go ---
Write-Log "Checking for Go..."
if (-not (Test-Command "go")) {
    Write-Error-And-Exit "Go is not installed or not in PATH. Install Go $MinGoVersion+ from https://go.dev/dl/"
}

$goVersionOutput = & go version
if ($goVersionOutput -match "go(\d+\.\d+)") {
    $goVersion = [Version]$Matches[1]
} else {
    Write-Error-And-Exit "Could not parse Go version from: $goVersionOutput"
}

if ($goVersion -lt $MinGoVersion) {
    Write-Error-And-Exit "Go $MinGoVersion+ is required, found $goVersion"
}
Write-Log "Found Go $goVersion"

# Ensure GOBIN is in PATH for installed tools
$goBin = & go env GOBIN
if ([string]::IsNullOrEmpty($goBin)) {
    $goBin = Join-Path (& go env GOPATH) "bin"
}
if ($env:PATH -notlike "*$goBin*") {
    $env:PATH = "$goBin;$env:PATH"
}

# --- Install buf ---
Write-Log "Installing buf (protobuf toolchain)..."
if (Test-Command "buf") {
    $bufVersion = & buf --version
    Write-Log "buf already installed: $bufVersion"
} else {
    Invoke-Step "go install github.com/bufbuild/buf/cmd/buf@latest" {
        & go install github.com/bufbuild/buf/cmd/buf@latest
        if ($LASTEXITCODE -ne 0) { Write-Error-And-Exit "Failed to install buf" }
        $bufVersion = & buf --version
        Write-Log "buf installed: $bufVersion"
    }
}

# --- Install task ---
Write-Log "Installing task (task runner)..."
if (Test-Command "task") {
    $taskVersion = & task --version
    Write-Log "task already installed: $taskVersion"
} else {
    Invoke-Step "go install github.com/go-task/task/v3/cmd/task@latest" {
        & go install github.com/go-task/task/v3/cmd/task@latest
        if ($LASTEXITCODE -ne 0) { Write-Error-And-Exit "Failed to install task" }
        $taskVersion = & task --version
        Write-Log "task installed: $taskVersion"
    }
}

# --- Download Go module dependencies ---
Write-Log "Downloading Go module dependencies..."
Invoke-Step "go mod download" {
    & go mod download
    if ($LASTEXITCODE -ne 0) { Write-Error-And-Exit "Failed to download Go dependencies" }
}

# --- Generate protobuf code ---
Write-Log "Generating protobuf code..."
Invoke-Step "buf generate" {
    Push-Location api
    try {
        & buf generate
        if ($LASTEXITCODE -ne 0) { Write-Error-And-Exit "Failed to generate protobuf code" }
    } finally {
        Pop-Location
    }
}

Write-Log ""
Write-Log "Setup complete. Available commands:"
Write-Log "  task          Build everything (generate + build)"
Write-Log "  task build    Build binaries"
Write-Log "  task test     Run unit tests"
Write-Log "  task test:all Run all tests"
Write-Log "  task --list   Show all available tasks"

$goPath = & go env GOPATH
Write-Log ""
Write-Log "Make sure $(Join-Path $goPath 'bin') is in your PATH to use buf and task."
