# ============================================================================
# build.ps1 — Windows PowerShell 构建脚本
# 功能：编译 Windows 二进制并部署到 code-text/ 目录
#
# 用法：
#   .\build.ps1                  # 构建并部署到 code-text/
#   .\build.ps1 -Clean           # 清理后构建
#   .\build.ps1 -Test            # 运行测试
#   .\build.ps1 -Lint            # 运行 golangci-lint
#   .\build.ps1 -Vet             # 运行 go vet
#   .\build.ps1 -Fmt             # 格式化代码
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

# ---- 变量 ------------------------------------------------------------------
$BinaryName = "code-context-mcp"
$BinaryExe  = "$BinaryName.exe"
$DeployDir  = "code-text"

# 版本信息
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

# ---- 颜色输出 ---------------------------------------------------------------
function Write-Info  { Write-Host "[INFO]  $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "[WARN]  $args" -ForegroundColor Yellow }
function Write-ErrorMsg { Write-Host "[ERROR] $args" -ForegroundColor Red }

# ---- 帮助 -------------------------------------------------------------------
function Show-Help {
    Write-Host "Usage: .\build.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  (none)    构建并部署到 $DeployDir\"
    Write-Host "  -Clean    清理构建产物后重新构建"
    Write-Host "  -Test     运行测试"
    Write-Host "  -Lint     运行 golangci-lint"
    Write-Host "  -Vet      运行 go vet"
    Write-Host "  -Fmt      格式化代码"
    Write-Host "  -Help     显示此帮助"
}

if ($Help) {
    Show-Help
    exit 0
}

# ---- 清理 -------------------------------------------------------------------
function Invoke-Clean {
    Write-Info "清理构建产物..."
    Remove-Item -Path $BinaryExe, "cmd/$BinaryName/$BinaryExe" -ErrorAction SilentlyContinue
    Remove-Item -Path "coverage.out", "coverage.html" -ErrorAction SilentlyContinue
    Remove-Item -Path $DeployDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Info "清理完成"
}

# ---- 测试 -------------------------------------------------------------------
function Invoke-Test {
    Write-Info "运行测试..."
    go test -v -race -coverprofile=coverage.out ./...
    if ($LASTEXITCODE -ne 0) { throw "测试失败" }
    Write-Info "测试通过"
}

# ---- Lint -------------------------------------------------------------------
function Invoke-Lint {
    Write-Info "运行 golangci-lint..."
    if (-not (Get-Command "golangci-lint" -ErrorAction SilentlyContinue)) {
        Write-Info "安装 golangci-lint..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    }
    golangci-lint run ./...
    if ($LASTEXITCODE -ne 0) { throw "Lint 检测到问题" }
    Write-Info "Lint 通过"
}

# ---- Vet --------------------------------------------------------------------
function Invoke-Vet {
    Write-Info "运行 go vet..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet 检测到问题" }
    Write-Info "go vet 通过"
}

# ---- Fmt --------------------------------------------------------------------
function Invoke-Fmt {
    Write-Info "格式化代码..."
    go fmt ./...
    Write-Info "格式化完成"
}

# ---- 构建 -------------------------------------------------------------------
function Invoke-Build {
    Write-Info "构建 $BinaryExe..."
    Write-Info "  版本: $Version"
    Write-Info "  Commit: $Commit"
    Write-Info "  构建日期: $Date"

    $env:CGO_ENABLED = "0"
    go build -v -trimpath -ldflags="$LdFlags" -o "$BinaryExe" ./cmd/code-context-mcp
    if ($LASTEXITCODE -ne 0) { throw "构建失败" }

    Write-Info "构建成功: $BinaryExe"
}

# ---- 部署 -------------------------------------------------------------------
function Invoke-Deploy {
    Write-Info "部署到 $DeployDir\..."

    if (-not (Test-Path $DeployDir)) {
        New-Item -ItemType Directory -Path $DeployDir | Out-Null
    }

    Copy-Item -Path $BinaryExe -Destination "$DeployDir\$BinaryExe" -Force
    Write-Info "  - $DeployDir\$BinaryExe"

    # 生成版本占位替换后的 start-mcp.sh
    $templatePath = "start-mcp.sh.template"
    if (Test-Path $templatePath) {
        $template = Get-Content $templatePath -Raw
        $buildDate = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
        $content = $template -replace "__VERSION__", $Version
        $content = $content -replace "__BUILD_DATE__", $buildDate
        Set-Content -Path "$DeployDir\start-mcp.sh" -Value $content -NoNewline
        Write-Info "  - $DeployDir\start-mcp.sh"
    }

    Write-Info "部署完成"
}

# ---- 主流程 -----------------------------------------------------------------
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
        # 默认：构建 + 部署
        Invoke-Build
        Invoke-Deploy
        Write-Info ""
        Write-Info "构建和部署完成!"
        Write-Info "输出目录: $DeployDir\"
        Write-Info "可执行文件: $DeployDir\$BinaryExe"
    }
} catch {
    Write-ErrorMsg $_.Exception.Message
    exit 1
}
