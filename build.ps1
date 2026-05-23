# ============================================================================
# build.ps1 鈥?Windows PowerShell 鏋勫缓鑴氭湰
# 鍔熻兘锛氱紪璇?Windows 浜岃繘鍒跺苟閮ㄧ讲鍒?code-text/ 鐩綍
#
# 鐢ㄦ硶锛?
#   .\build.ps1                  # 鏋勫缓骞堕儴缃插埌 code-text/
#   .\build.ps1 -Clean           # 娓呯悊鍚庢瀯寤?
#   .\build.ps1 -Test            # 杩愯娴嬭瘯
#   .\build.ps1 -Lint            # 杩愯 golangci-lint
#   .\build.ps1 -Vet             # 杩愯 go vet
#   .\build.ps1 -Fmt             # 鏍煎紡鍖栦唬鐮?
# ============================================================================

# 纭繚鎺у埗鍙颁互 UTF-8 杈撳嚭涓枃锛岄伩鍏嶄贡鐮?
[Console]::OutputEncoding = [Text.Encoding]::UTF8

param(
    [switch] $Clean,
    [switch] $Test,
    [switch] $Lint,
    [switch] $Vet,
    [switch] $Fmt,
    [switch] $Help
)

$ErrorActionPreference = "Stop"

# ---- 鍙橀噺 ------------------------------------------------------------------
$BinaryName = "code-context-mcp"
$BinaryExe  = "$BinaryName.exe"
$DeployDir  = "code-text"

# 鐗堟湰淇℃伅
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

# ---- 棰滆壊杈撳嚭 ---------------------------------------------------------------
function Write-Info  { Write-Host "[INFO]  $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "[WARN]  $args" -ForegroundColor Yellow }
function Write-ErrorMsg { Write-Host "[ERROR] $args" -ForegroundColor Red }

# ---- 甯姪 -------------------------------------------------------------------
function Show-Help {
    Write-Host "Usage: .\build.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  (none)    鏋勫缓骞堕儴缃插埌 $DeployDir\"
    Write-Host "  -Clean    娓呯悊鏋勫缓浜х墿鍚庨噸鏂版瀯寤?
    Write-Host "  -Test     杩愯娴嬭瘯"
    Write-Host "  -Lint     杩愯 golangci-lint"
    Write-Host "  -Vet      杩愯 go vet"
    Write-Host "  -Fmt      鏍煎紡鍖栦唬鐮?
    Write-Host "  -Help     鏄剧ず姝ゅ府鍔?
}

if ($Help) {
    Show-Help
    exit 0
}

# ---- 娓呯悊 -------------------------------------------------------------------
function Invoke-Clean {
    Write-Info "娓呯悊鏋勫缓浜х墿..."
    Remove-Item -Path $BinaryExe, "cmd/$BinaryName/$BinaryExe" -ErrorAction SilentlyContinue
    Remove-Item -Path "coverage.out", "coverage.html" -ErrorAction SilentlyContinue
    Remove-Item -Path $DeployDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Info "娓呯悊瀹屾垚"
}

# ---- 娴嬭瘯 -------------------------------------------------------------------
function Invoke-Test {
    Write-Info "杩愯娴嬭瘯..."
    go test -v -race -coverprofile=coverage.out ./...
    if ($LASTEXITCODE -ne 0) { throw "娴嬭瘯澶辫触" }
    Write-Info "娴嬭瘯閫氳繃"
}

# ---- Lint -------------------------------------------------------------------
function Invoke-Lint {
    Write-Info "杩愯 golangci-lint..."
    if (-not (Get-Command "golangci-lint" -ErrorAction SilentlyContinue)) {
        Write-Info "瀹夎 golangci-lint..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    }
    golangci-lint run ./...
    if ($LASTEXITCODE -ne 0) { throw "Lint 妫€娴嬪埌闂" }
    Write-Info "Lint 閫氳繃"
}

# ---- Vet --------------------------------------------------------------------
function Invoke-Vet {
    Write-Info "杩愯 go vet..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet 妫€娴嬪埌闂" }
    Write-Info "go vet 閫氳繃"
}

# ---- Fmt --------------------------------------------------------------------
function Invoke-Fmt {
    Write-Info "鏍煎紡鍖栦唬鐮?.."
    go fmt ./...
    Write-Info "鏍煎紡鍖栧畬鎴?
}

# ---- 鏋勫缓 -------------------------------------------------------------------
function Invoke-Build {
    Write-Info "鏋勫缓 $BinaryExe..."
    Write-Info "  鐗堟湰: $Version"
    Write-Info "  Commit: $Commit"
    Write-Info "  鏋勫缓鏃ユ湡: $Date"

    $env:CGO_ENABLED = "0"
    go build -v -trimpath -ldflags="$LdFlags" -o "$BinaryExe" ./cmd/code-context-mcp
    if ($LASTEXITCODE -ne 0) { throw "鏋勫缓澶辫触" }

    Write-Info "鏋勫缓鎴愬姛: $BinaryExe"
}

# ---- 閮ㄧ讲 -------------------------------------------------------------------
function Invoke-Deploy {
    Write-Info "閮ㄧ讲鍒?$DeployDir\..."

    if (-not (Test-Path $DeployDir)) {
        New-Item -ItemType Directory -Path $DeployDir | Out-Null
    }

    Copy-Item -Path $BinaryExe -Destination "$DeployDir\$BinaryExe" -Force
    Write-Info "  - $DeployDir\$BinaryExe"

    # 鐢熸垚鐗堟湰鍗犱綅鏇挎崲鍚庣殑 start-mcp.sh
    $templatePath = "start-mcp.sh.template"
    if (Test-Path $templatePath) {
        $template = Get-Content $templatePath -Raw
        $buildDate = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
        $content = $template -replace "__VERSION__", $Version
        $content = $content -replace "__BUILD_DATE__", $buildDate
        Set-Content -Path "$DeployDir\start-mcp.sh" -Value $content -NoNewline
        Write-Info "  - $DeployDir\start-mcp.sh"
    }

    Write-Info "閮ㄧ讲瀹屾垚"
}

# ---- 涓绘祦绋?-----------------------------------------------------------------
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
        # 榛樿锛氭瀯寤?+ 閮ㄧ讲
        Invoke-Build
        Invoke-Deploy
        Write-Info ""
        Write-Info "鏋勫缓鍜岄儴缃插畬鎴?"
        Write-Info "杈撳嚭鐩綍: $DeployDir\"
        Write-Info "鍙墽琛屾枃浠? $DeployDir\$BinaryExe"
    }
} catch {
    Write-ErrorMsg $_.Exception.Message
    exit 1
}
