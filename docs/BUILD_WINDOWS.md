# Windows PowerShell 构建脚本

## 概述

`build.ps1` 是一个 PowerShell 脚本，提供了与 Unix/Linux 系统上 `Makefile` 相同的功能，适用于 Windows 环境。

## 功能对应表

| Makefile 命令 | PowerShell 命令 | 描述 |
|--------------|----------------|------|
| `make help` | `.\build.ps1 help` | 显示帮助信息 |
| `make build` | `.\build.ps1 build` | 构建二进制文件到 `code-context-mcp/` 目录 |
| `make test` | `.\build.ps1 test` | 运行测试 |
| `make clean` | `.\build.ps1 clean` | 清理构建产物 |
| `make lint` | `.\build.ps1 lint` | 运行 golangci-lint |
| `make vet` | `.\build.ps1 vet` | 运行 go vet |
| `make fmt` | `.\build.ps1 fmt` | 格式化代码 |
| `make version` | `.\build.ps1 version` | 显示版本信息 |
| `make dev` | `.\build.ps1 dev` | 构建并运行开发模式 |
| `make test-coverage` | `.\build.ps1 test-coverage` | 运行测试并生成覆盖率报告 |
| `make install` | `.\build.ps1 install` | 安装到 GOPATH/bin |

## 使用示例

### 1. 显示帮助信息
```powershell
.\build.ps1 help
```

### 2. 构建项目
```powershell
.\build.ps1 build
```
构建成功后，可执行文件将生成在 `code-context-mcp\code-context-mcp.exe`（或 `code-context-mcp\code-context-mcp`）

### 3. 运行测试
```powershell
.\build.ps1 test
```

### 4. 清理构建产物
```powershell
.\build.ps1 clean
```

### 5. 构建并运行开发模式
```powershell
.\build.ps1 dev
```

### 6. 运行所有检查和格式化
```powershell
.\build.ps1 fmt
.\build.ps1 vet
.\build.ps1 lint
```

## 环境要求

1. **PowerShell 5.1 或更高版本**
2. **Go 1.21 或更高版本**
3. **Git**（用于获取版本信息，可选）

## 执行策略

首次运行 PowerShell 脚本可能需要更改执行策略：

```powershell
# 查看当前执行策略
Get-ExecutionPolicy

# 设置为 RemoteSigned（推荐）
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser

# 或设置为 Bypass（仅当前会话）
Set-ExecutionPolicy Bypass -Scope Process
```

## 与 Makefile 的差异

1. **路径分隔符**：使用 Windows 路径分隔符 `\`
2. **文件扩展名**：Windows 上生成 `.exe` 扩展名
3. **环境变量**：使用 PowerShell 的 `$env:` 语法
4. **命令执行**：使用 PowerShell 的 `&` 操作符执行外部命令

## 故障排除

### 1. "无法加载文件...因为在此系统上禁止运行脚本"
```powershell
# 以管理员身份运行 PowerShell，然后执行：
Set-ExecutionPolicy RemoteSigned
```

### 2. "go: 无法识别命令"
确保 Go 已正确安装并添加到 PATH 环境变量。

### 3. "golangci-lint: 无法识别命令"
脚本会自动安装 golangci-lint，如果失败请手动安装：
```powershell
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
```

### 4. 权限问题
如果遇到权限问题，以管理员身份运行 PowerShell。

## 注意事项

1. 脚本会自动创建输出目录 `code-context-mcp\`
2. 版本信息从 Git 仓库获取，如果没有 Git 则使用默认值
3. 构建时自动禁用 CGO（`CGO_ENABLED=0`）
4. 清理操作会删除 `code-context-mcp\` 目录和所有构建产物

## 与 Makefile 的功能对比

所有 Makefile 的功能都在 PowerShell 脚本中实现，包括：
- 版本信息嵌入（通过 LDFLAGS）
- 自动安装 golangci-lint
- 测试覆盖率报告生成
- 开发模式构建和运行