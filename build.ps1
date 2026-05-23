# ============================================================================
# build.ps1 — Windows PowerShell build script
# Builds code-context-mcp.exe for Windows amd64.
#
# Usage:
#   .\build.ps1              # Build and deploy to code-text\
#   .\build.ps1 -Clean       # Clean then build
#   .\build.ps1 -Test        # Run tests
#   .\build.ps1 -Lint        # Run golangci-lint
#   .\build.ps1 -Vet         # Run go vet
#   .\build.ps1 -Fmt         # Format code
#   .\build.ps1 -Help        # Show this help
# ============================================================================

param(
    [switch] $Clean,
    [switch] $Test,
    [switch] $Lint,
    [switch] $Vet,
    [switch] $Fmt,
    [switch] $Help
)

$ErrorActionPreference = "Stop"

# ---- Variables --------------------------------------------------------------
$BinaryName  = "code-context-mcp"
$BinaryExe   = "$BinaryName.exe"
$DeployDir   = "code-text"

# Version info (from git, fallback to "dev")
try {
    $Version = (git describe --tags --always --dirty 2>$null) -replace "`n|`r", ""
    if (-not $Version) { $Version = "dev" }
    $Commit  = (git rev-parse --short HEAD 2>$null) -replace "`n|`r", ""
    if (-not $Commit) { $Commit = "unknown" }
    $Date    = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ").ToUniversalTime().ToString("yyyy-MM-dd_HH:mm:ss")
} catch {
    $Version = "dev"
    $Commit  = "unknown"
    $Date    = (Get-Date -Format "yyyy-MM-dd_HH:mm:ss")
}

$LdFlags = "-s -w -X main.version=$Version -X main.commit=$Commit -X main.date=$Date"

# ---- Output helpers ---------------------------------------------------------
function Write-Info  { Write-Host "[INFO]  $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "[WARN]  $args" -ForegroundColor Yellow }
function Write-ErrorMsg { Write-Host "[ERROR] $args" -ForegroundColor Red }

# ---- Help -------------------------------------------------------------------
function Show-Help {
    Write-Host "Usage: .\build.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  (none)    Build and deploy to $DeployDir\"
    Write-Host "  -Clean    Clean artifacts then rebuild"
    Write-Host "  -Test     Run tests"
    Write-Host "  -Lint     Run golangci-lint"
    Write-Host "  -Vet      Run go vet"
    Write-Host "  -Fmt      Format code"
    Write-Host "  -Help     Show this help"
}

if ($Help) {
    Show-Help
    exit 0
}

# ---- Clean ------------------------------------------------------------------
function Invoke-Clean {
    Write-Info "Cleaning build artifacts..."
    Remove-Item -Path $BinaryExe, "cmd/$BinaryName/$BinaryExe" -ErrorAction SilentlyContinue
    Remove-Item -Path "coverage.out", "coverage.html" -ErrorAction SilentlyContinue
    Remove-Item -Path $DeployDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Info "Clean done"
}

# ---- Test -------------------------------------------------------------------
function Invoke-Test {
    Write-Info "Running tests..."
    go test -v -race -coverprofile=coverage.out ./...
    if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
    Write-Info "All tests passed"
}

# ---- Lint -------------------------------------------------------------------
function Invoke-Lint {
    Write-Info "Running golangci-lint..."
    if (-not (Get-Command "golangci-lint" -ErrorAction SilentlyContinue)) {
        Write-Info "Installing golangci-lint..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    }
    golangci-lint run ./...
    if ($LASTEXITCODE -ne 0) { throw "Lint found issues" }
    Write-Info "Lint passed"
}

# ---- Vet --------------------------------------------------------------------
function Invoke-Vet {
    Write-Info "Running go vet..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet found issues" }
    Write-Info "go vet passed"
}

# ---- Fmt --------------------------------------------------------------------
function Invoke-Fmt {
    Write-Info "Formatting code..."
    go fmt ./...
    Write-Info "Format done"
}

# ---- Build ------------------------------------------------------------------
function Invoke-Build {
    Write-Info "Building $BinaryExe for Windows amd64..."
    Write-Info "  Version: $Version"
    Write-Info "  Commit:  $Commit"
    Write-Info "  Date:    $Date"

    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -v -trimpath -ldflags="$LdFlags" -o "$BinaryExe" ./cmd/code-context-mcp
    if ($LASTEXITCODE -ne 0) { throw "Build failed" }

    Write-Info "Build succeeded: $BinaryExe"
}

# ---- Deploy -----------------------------------------------------------------
function Invoke-Deploy {
    Write-Info "Deploying to $DeployDir\..."
    if (-not (Test-Path $DeployDir)) {
        New-Item -ItemType Directory -Path $DeployDir | Out-Null
    }
    Copy-Item -Path $BinaryExe -Destination "$DeployDir\$BinaryExe" -Force
    Write-Info "  - $DeployDir\$BinaryExe"

    # Generate start-mcp.sh from template (replaces __VERSION__ / __BUILD_DATE__)
    $templatePath = "start-mcp.sh.template"
    if (Test-Path $templatePath) {
        $template = Get-Content $templatePath -Raw
        $buildDate = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
        $content = $template -replace "__VERSION__", $Version
        $content = $content -replace "__BUILD_DATE__", $buildDate
        Set-Content -Path "$DeployDir\start-mcp.sh" -Value $content -NoNewline
        Write-Info "  - $DeployDir\start-mcp.sh"
    }

    Write-Info "Deploy done"
}

# ---- Main -------------------------------------------------------------------
try {
    if ($Clean) {
        Invoke-Clean
    }

    if ($Test) {
        Invoke-Test
    } elseif ($Lint) {
        Invoke-Lint
    } elseif ($Vet) {
        Invoke-Vet
    } elseif ($Fmt) {
        Invoke-Fmt
    } else {
        Invoke-Build
        Invoke-Deploy
        Write-Info ""
        Write-Info "Build & deploy complete."
        Write-Info "Output dir: $DeployDir\"
        Write-Info "Binary:     $DeployDir\$BinaryExe"
    }
} catch {
    Write-ErrorMsg $_.Exception.Message
    exit 1
}
