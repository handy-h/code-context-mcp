# PowerShell 构建脚本 for code-context-mcp
# 对应 Makefile 的功能

param(
    [string]$Target = "help"
)

# 变量定义
$BINARY_NAME = "code-context-mcp"
$BINARY_PATH = "cmd/code-context-mcp/${BINARY_NAME}"
$OUTPUT_PATH = "${BINARY_NAME}/${BINARY_NAME}"
$GOLANGCI_LINT_VERSION = "v1.64.8"

# 获取版本信息
function Get-VersionInfo {
    $version = "dev"
    $commit = ""
    $date = Get-Date -Format "yyyy-MM-dd_HH:mm:ss"
    
    # 尝试获取 git 版本信息
    try {
        $gitVersion = git describe --tags --always --dirty 2>$null
        if ($LASTEXITCODE -eq 0) {
            $version = $gitVersion
        }
        
        $gitCommit = git rev-parse --short HEAD 2>$null
        if ($LASTEXITCODE -eq 0) {
            $commit = $gitCommit
        }
    } catch {
        # git 命令失败，使用默认值
    }
    
    return @{
        Version = $version
        Commit = $commit
        Date = $date
    }
}

# 显示帮助信息
function Show-Help {
    Write-Host "Usage: .\build.ps1 [target]"
    Write-Host ""
    Write-Host "Targets:"
    Write-Host "  help        - 显示此帮助信息"
    Write-Host "  build       - 构建二进制文件"
    Write-Host "  test        - 运行测试"
    Write-Host "  clean       - 清理构建产物"
    Write-Host "  lint        - 运行 linters"
    Write-Host "  vet         - 运行 go vet"
    Write-Host "  fmt         - 格式化代码"
    Write-Host "  version     - 显示版本信息"
    Write-Host "  dev         - 构建并运行开发模式（索引当前目录）"
    Write-Host ""
    Write-Host "示例:"
    Write-Host "  .\build.ps1 build"
    Write-Host "  .\build.ps1 test"
}

# 构建二进制文件
function Invoke-Build {
    Write-Host "Building ${BINARY_NAME}..."
    
    # 创建输出目录
    if (-not (Test-Path $BINARY_NAME)) {
        New-Item -ItemType Directory -Path $BINARY_NAME -Force | Out-Null
    }
    
    # 获取版本信息
    $versionInfo = Get-VersionInfo
    $ldflags = "-s -w -X main.version=$($versionInfo.Version) -X main.commit=$($versionInfo.Commit) -X main.date=$($versionInfo.Date)"
    
    # 构建
    $env:CGO_ENABLED = "0"
    go build -v -trimpath -ldflags="$ldflags" -o $OUTPUT_PATH ./cmd/code-context-mcp
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "构建成功！可执行文件位于: $OUTPUT_PATH" -ForegroundColor Green
    } else {
        Write-Host "构建失败！" -ForegroundColor Red
        exit 1
    }
}

# 运行测试
function Invoke-Test {
    Write-Host "Running tests..."
    go test -v -race -coverprofile=coverage.out ./...
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "测试通过！" -ForegroundColor Green
    } else {
        Write-Host "测试失败！" -ForegroundColor Red
        exit 1
    }
}

# 清理构建产物
function Invoke-Clean {
    Write-Host "Cleaning..."
    
    # 删除文件
    $filesToRemove = @(
        $BINARY_PATH,
        "${BINARY_PATH}.exe",
        $OUTPUT_PATH,
        "${OUTPUT_PATH}.exe",
        "coverage.out",
        "coverage.html"
    )
    
    foreach ($file in $filesToRemove) {
        if (Test-Path $file) {
            Remove-Item $file -Force
            Write-Host "已删除: $file"
        }
    }
    
    # 删除目录
    if (Test-Path $BINARY_NAME) {
        Remove-Item $BINARY_NAME -Recurse -Force
        Write-Host "已删除目录: $BINARY_NAME"
    }
    
    Write-Host "清理完成！" -ForegroundColor Green
}

# 运行 linters
function Invoke-Lint {
    Write-Host "Running golangci-lint ${GOLANGCI_LINT_VERSION}..."
    
    # 检查 golangci-lint 是否已安装
    $lintCmd = Get-Command golangci-lint -ErrorAction SilentlyContinue
    if ($null -eq $lintCmd) {
        Write-Host "golangci-lint 未安装。正在安装 ${GOLANGCI_LINT_VERSION}..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}
        
        if ($LASTEXITCODE -ne 0) {
            Write-Host "安装 golangci-lint 失败！" -ForegroundColor Red
            exit 1
        }
        
        # 添加到 PATH
        $goBinPath = (go env GOPATH) + "\bin"
        $env:Path += ";$goBinPath"
    }
    
    golangci-lint run ./...
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Lint 检查通过！" -ForegroundColor Green
    } else {
        Write-Host "Lint 检查失败！" -ForegroundColor Red
        exit 1
    }
}

# 运行 go vet
function Invoke-Vet {
    Write-Host "Running go vet..."
    go vet ./...
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Vet 检查通过！" -ForegroundColor Green
    } else {
        Write-Host "Vet 检查失败！" -ForegroundColor Red
        exit 1
    }
}

# 格式化代码
function Invoke-Fmt {
    Write-Host "Formatting code..."
    go fmt ./...
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "代码格式化完成！" -ForegroundColor Green
    } else {
        Write-Host "代码格式化失败！" -ForegroundColor Red
        exit 1
    }
}

# 显示版本信息
function Invoke-Version {
    $versionInfo = Get-VersionInfo
    Write-Host "Version: $($versionInfo.Version)"
    Write-Host "Commit: $($versionInfo.Commit)"
    Write-Host "Date: $($versionInfo.Date)"
}

# 构建并运行开发模式
function Invoke-Dev {
    # 先构建
    Invoke-Build
    
    # 运行开发模式
    Write-Host "Running in development mode..."
    & "./${OUTPUT_PATH}" -index .
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "运行失败！" -ForegroundColor Red
        exit 1
    }
}

# 生成测试覆盖率报告
function Invoke-TestCoverage {
    Invoke-Test
    Write-Host "Generating coverage report..."
    go tool cover -html=coverage.out -o coverage.html
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "覆盖率报告已生成: coverage.html" -ForegroundColor Green
    } else {
        Write-Host "生成覆盖率报告失败！" -ForegroundColor Red
        exit 1
    }
}

# 安装到 GOPATH/bin
function Invoke-Install {
    Write-Host "Installing ${BINARY_NAME}..."
    
    # 获取版本信息
    $versionInfo = Get-VersionInfo
    $ldflags = "-s -w -X main.version=$($versionInfo.Version) -X main.commit=$($versionInfo.Commit) -X main.date=$($versionInfo.Date)"
    
    # 安装
    $env:CGO_ENABLED = "0"
    go install -v -trimpath -ldflags="$ldflags" ./cmd/code-context-mcp
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "安装成功！" -ForegroundColor Green
    } else {
        Write-Host "安装失败！" -ForegroundColor Red
        exit 1
    }
}

# 主函数
function Main {
    switch ($Target.ToLower()) {
        "help" {
            Show-Help
        }
        "build" {
            Invoke-Build
        }
        "test" {
            Invoke-Test
        }
        "clean" {
            Invoke-Clean
        }
        "lint" {
            Invoke-Lint
        }
        "vet" {
            Invoke-Vet
        }
        "fmt" {
            Invoke-Fmt
        }
        "version" {
            Invoke-Version
        }
        "dev" {
            Invoke-Dev
        }
        "test-coverage" {
            Invoke-TestCoverage
        }
        "install" {
            Invoke-Install
        }
        default {
            Write-Host "未知的目标: $Target" -ForegroundColor Red
            Write-Host ""
            Show-Help
            exit 1
        }
    }
}

# 执行主函数
Main

