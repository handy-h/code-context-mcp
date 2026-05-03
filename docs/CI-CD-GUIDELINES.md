# CI/CD 流程规范与注意事项

本文档记录了项目 CI/CD 流程中遇到的所有规范问题和解决方案，供后续开发参考。

---

## 1. Go 代码规范

### 1.1 禁止使用已弃用的 `io/ioutil` 包

自 Go 1.16 起，`io/ioutil` 包已弃用（Go 1.19 起静态检查会报错）。必须使用替代包：

| 弃用 API | 替代 API |
|---|---|
| `ioutil.ReadFile(path)` | `os.ReadFile(path)` |
| `ioutil.ReadAll(r)` | `io.ReadAll(r)` |
| `ioutil.WriteFile(path, data, perm)` | `os.WriteFile(path, data, perm)` |
| `ioutil.TempDir(dir, pattern)` | `os.MkdirTemp(dir, pattern)` |
| `ioutil.TempFile(dir, pattern)` | `os.CreateTemp(dir, pattern)` |

**示例**：
```go
// 错误
import "io/ioutil"
data, err := ioutil.ReadFile("config.json")

// 正确
import "os"
data, err := os.ReadFile("config.json")
```

### 1.2 错误字符串不得以大写字母开头（ST1005）

Go 静态检查规则 ST1005 要求 `fmt.Errorf` 和 `errors.New` 中的错误字符串不以大写字母开头（专有名词缩写如 JSON、HTTP 也应小写）。

```go
// 错误
fmt.Errorf("Ollama request failed: %v", err)
fmt.Errorf("JSON parsing failed: %v", err)

// 正确
fmt.Errorf("ollama request failed: %v", err)
fmt.Errorf("json parsing failed: %v", err)
```

### 1.3 禁止声明未使用的变量和类型（U1000）

未使用的变量、类型、函数会被静态检查器报告为 U1000 错误。如果暂时不需要，应注释掉或删除。

```go
// 错误：goPackageRe 和 goImportRe 未使用
var (
    goPackageRe = regexp.MustCompile(`^package\s+`)
    goImportRe  = regexp.MustCompile(`^import\s+`)
)

// 正确：注释掉或删除
// var goPackageRe = regexp.MustCompile(`^package\s+`)
// var goImportRe  = regexp.MustCompile(`^import\s+`)
```

### 1.4 导入的包必须被使用

`go vet` 会检查未使用的导入。替换 `io/ioutil` 时，务必同步更新 import 声明，移除不再需要的包（如 `io`）。

---

## 2. GitHub Actions 规范

### 2.1 Node.js 20 已弃用

GitHub Actions 正在从 Node.js 20 迁移到 Node.js 24。使用 Node.js 20 的 actions 会产生弃用警告。

**受影响的 actions**：
- `actions/checkout@v4`
- `actions/setup-go@v5`
- `actions/upload-artifact@v4`
- `actions/cache`（由 setup-go 内部使用）

**解决方案**：在工作流中添加环境变量强制使用 Node.js 24：

```yaml
jobs:
  build:
    env:
      FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true
```

**时间线**：
- 2026年6月2日：Node.js 24 成为默认版本
- 2026年9月16日：Node.js 20 从 runner 中移除

### 2.2 Windows 运行器使用 PowerShell

GitHub Actions 的 Windows 运行器默认使用 **PowerShell**，不是 bash。以下语法在 PowerShell 中无效：

```yaml
# 错误：PowerShell 不识别此语法
- run: |
    GOOS=windows
    GOARCH=amd64
    go build -o app .
```

**解决方案**：为步骤显式指定 `shell: bash`：

```yaml
- name: Build
  shell: bash
  run: |
    export GOOS=windows
    export GOARCH=amd64
    go build -o app .
```

**注意**：`export` 关键字在 bash 中是必需的，否则环境变量不会传递给后续命令。

### 2.3 工作流权限声明

如果工作流需要推送 Docker 镜像到 GitHub Container Registry (ghcr.io)，必须声明 `packages: write` 权限：

```yaml
permissions:
  contents: read
  packages: write
```

否则会报错：`denied: installation not allowed to Create organization package`。

### 2.4 Docker 推送可能因组织权限被拒绝

即使声明了 `packages: write`，组织级别的设置可能仍然禁止 Actions 创建包。建议为 Docker 构建步骤添加 `continue-on-error: true`，避免 Docker 推送失败导致整个工作流失败：

```yaml
- name: Build and push Docker image
  uses: docker/build-push-action@v5
  with:
    push: true
    tags: ghcr.io/${{ github.repository }}:latest
  continue-on-error: true
```

---

## 3. GoReleaser 规范

### 3.1 配置文件必须声明 `version: 2`

GoReleaser v2 要求配置文件声明版本号，否则会报错：

```
only version: 2 configuration files are supported, yours is version: 0
```

**正确配置**：
```yaml
# .goreleaser.yml
version: 2

project_name: code-context-mcp
# ...
```

### 3.2 `dockers` 配置已弃用

GoReleaser v2 中 `dockers` 和 `docker_manifests` 正在被 `dockers_v2` 替代。但 `dockers_v2` 的语法与 `dockers` 不同：

| `dockers` (旧) | `dockers_v2` (新) |
|---|---|
| `image_templates` | `image` |
| `use: buildx` | 不需要 |
| `build_flag_templates` | `platforms` + `labels` |

**注意**：`dockers_v2` 的 `image` 字段格式与 `image_templates` 不同，使用前请查阅 [GoReleaser 文档](https://goreleaser.com/customization/docker/)。

### 3.3 `archives.format` 已弃用

GoReleaser v2 中 `archives.format` 字段已弃用，默认使用 `tar.gz` 格式。移除此字段即可。

### 3.4 `snapshot.name_template` 已弃用

GoReleaser v2 中 `snapshot.name_template` 已弃用，使用默认命名即可。

### 3.5 Docker 多平台构建的 manifest list 问题

使用 `dockers` 配置的 `use: buildx` + `--platform=linux/amd64,linux/arm64` 时，可能遇到：

```
docker exporter does not currently support exporting manifest lists
```

**解决方案**：
1. 使用 `dockers_v2` 配置（推荐）
2. 或暂时禁用 Docker 构建，仅发布二进制文件
3. 或使用 GitHub Actions 单独构建 Docker 镜像

---

## 4. Git 标签与发布规范

### 4.1 Release 工作流由标签触发

`release.yml` 配置为仅在推送 `v*` 标签时触发：

```yaml
on:
  push:
    tags: [ 'v*' ]
```

**创建并推送标签**：
```bash
git tag -a v0.0.1 -m "Release version 0.0.1"
git push origin v0.0.1
```

### 4.2 `git describe` 在无标签时的处理

构建脚本中使用 `git describe --tags --always --dirty` 获取版本号。在无标签的仓库中，此命令可能失败。应添加错误处理：

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
```

### 4.3 Docker 标签格式规范

Docker 镜像标签不得以 `-` 开头或包含无效字符。使用 `docker/metadata-action` 时，`type=sha,prefix={{branch}}-` 可能生成以 `-` 开头的标签（如 `main-abc123` 在某些上下文中变为 `-abc123`），导致：

```
invalid tag "ghcr.io/owner/repo:-882bc75": invalid reference format
```

**解决方案**：避免使用可能生成无效标签的 prefix 模式，或使用 `type=semver` 代替。

---

## 5. 静态检查工具

本项目 CI 使用以下静态检查工具：

| 工具 | 用途 | 常见错误 |
|---|---|---|
| `go vet` | 代码正确性检查 | 未使用的导入、错误格式 |
| `staticcheck` | 高级静态分析 | ST1005（错误字符串大写）、U1000（未使用代码）、SA1019（弃用 API） |

**本地运行**：
```bash
go vet ./...
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...
```

---

## 6. 完整的 CI/CD 工作流清单

| 工作流 | 触发条件 | 主要步骤 |
|---|---|---|
| `build.yml` | push to main/develop, tags, PR | 多平台构建、上传 artifact、Docker 多架构构建 |
| `test.yml` | push to main/develop, PR | 依赖验证、go vet、staticcheck、测试、覆盖率上传 |
| `release.yml` | push tags `v*` | GoReleaser 发布、Docker 镜像构建推送 |

---

## 7. 常见问题排查

### Q: Windows 构建失败，报 `GOOS=windows is not recognized`
**A**: Windows runner 默认使用 PowerShell，需为步骤添加 `shell: bash`。

### Q: 测试失败，报 `ST1005: error strings should not be capitalized`
**A**: 将 `fmt.Errorf` 中的错误消息首字母改为小写。

### Q: 测试失败，报 `SA1019: "io/ioutil" is deprecated`
**A**: 将 `io/ioutil` 替换为 `os` 和 `io` 包的对应函数。

### Q: GoReleaser 报 `only version: 2 configuration files are supported`
**A**: 在 `.goreleaser.yml` 顶部添加 `version: 2`。

### Q: Docker 推送报 `denied: installation not allowed to Create organization package`
**A**: 1) 确认工作流声明了 `permissions: packages: write`；2) 检查组织/仓库的 Actions 权限设置；3) 添加 `continue-on-error: true` 容错。

### Q: Release 工作流未触发
**A**: 确认已创建并推送了 `v*` 格式的标签。
