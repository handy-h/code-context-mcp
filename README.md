# code-context-mcp

为 AI 编码助手提供代码上下文的 MCP 服务，通过语义搜索、符号查找、影响分析等能力，减少上下文调用，替代 Grep/Glob/Read 的组合使用。

## 功能概览

| 工具 | 说明 | 典型场景 |
|------|------|---------|
| `code_search` | 语义搜索代码片段 | "处理用户认证的逻辑"、"配置加载相关代码" |
| `symbol_search` | 精确符号搜索 | 查找 `LoadConfig` 函数的所有引用 |
| `impact_analysis` | 影响范围分析 | 删除/重命名某函数后影响哪些文件 |
| `file_context` | 文件内容或结构摘要 | 先看摘要定位函数，再精确读取 |
| `index_project` | 手动索引构建 | 首次使用或代码大变更后 |

## 核心特性

- **自动索引**：MCP 启动时自动检测索引状态，搜索时发现过期自动后台增量更新，无需手动调用 `index_project`
- **按代码结构切分**：按函数/结构体/组件等语法边界切分，而非固定字符窗口，搜索结果语义完整
- **精确符号搜索**：基于倒排索引，支持驼峰/下划线模糊匹配，结果按文件分组含上下文摘要
- **影响范围分析**：一次调用完成 delete/rename/modify 的影响分析，返回修改建议
- **文件结构摘要**：`file_context(mode="summary")` 返回函数列表、类型列表、导入依赖，节省上下文窗口

## 快速开始

### 1. 安装前置依赖

**Ollama** — 本地嵌入模型服务

```bash
# 安装 Ollama（参考 https://ollama.com）
# 安装后拉取嵌入模型
ollama pull nomic-embed-text:latest
```

**Zilliz Cloud** — 向量数据库服务

- 注册 [Zilliz Cloud](https://cloud.zilliz.com/) 获取 URI 和 API Token
- 免费套餐即可满足个人项目需求

### 2. 克隆并构建

```bash
git clone https://github.com/handy-h/code-context-mcp.git
cd code-context-mcp
go build -o code-context-mcp .
```

### 3. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，填入你的 Zilliz Cloud 凭据和项目路径：

```env
ZILLIZ_URI=https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com
ZILLIZ_TOKEN=your_api_token_here
PROJECT_PATH=/path/to/your/project
```

### 4. 初始化 Zilliz 数据集

首次使用时，需要初始化 Zilliz Cloud 数据集。有两种方式：

**方式一：自动初始化（推荐）**
1. 确保 `AUTO_INDEX=true`（默认值）
2. 启动 MCP 服务（通过 AI 编程工具配置）
3. 服务启动时会自动检测并创建数据集（集合名为 `code-context`）

**方式二：手动初始化**
```bash
# 构建索引，自动创建数据集
./code-context-mcp -index /path/to/your/project
```

**验证数据集创建成功**：
1. 登录 [Zilliz Cloud 控制台](https://cloud.zilliz.com/)
2. 进入你的集群
3. 在 "Collections" 页面查看是否创建了名为 `code-context` 的集合

### 5. 配置 AI 编程工具

参考下方 [AI 编程工具 MCP 配置](#ai-编程工具-mcp-配置) 章节，将 `code-context-mcp` 添加到你的 AI 编程工具中。

## 配置

### 环境变量

通过 `.env` 文件（与可执行文件同目录）或系统环境变量配置：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `OLLAMA_URL` | `http://localhost:11434` | Ollama 服务地址 |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text:latest` | 嵌入模型名 |
| `EMBEDDING_DIM` | `768` | 向量维度 |
| `ZILLIZ_URI` | （必填） | Zilliz Cloud URI |
| `ZILLIZ_TOKEN` | （必填） | Zilliz Cloud API Token |
| `COLLECTION_NAME` | `code-context` | Milvus 集合名，首次使用时会自动创建 |
| `SCAN_EXTENSIONS` | `.go,.vue,.js,.ts,.py,.md` | 扫描的文件扩展名 |
| `CHUNK_SIZE` | `800` | 降级切分时的块大小（rune） |
| `MAX_CHUNK_SIZE` | `1500` | 结构切分后超长块的最大 rune 数 |
| `AUTO_INDEX` | `true` | 是否启用自动索引 |
| `PROJECT_PATH` | （空） | MCP 模式下自动索引的项目路径 |
| `INDEX_STATE_PATH` | （自动） | 索引状态文件路径，默认 `{PROJECT_PATH}/.code-context-index-state.json` |

### .env 文件模板

复制 `.env.example` 为 `.env` 并填入实际值：

```bash
cp .env.example .env
```

```env
# Ollama 嵌入模型配置
OLLAMA_URL=http://localhost:11434
OLLAMA_EMBED_MODEL=nomic-embed-text:latest
EMBEDDING_DIM=768

# Zilliz Cloud 向量数据库配置
ZILLIZ_URI=https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com
ZILLIZ_TOKEN=your_api_token_here
COLLECTION_NAME=code-context

# 索引配置
SCAN_EXTENSIONS=.go,.vue,.js,.ts,.py,.md
MAX_CHUNK_SIZE=1500

# 自动索引配置
AUTO_INDEX=true
PROJECT_PATH=/path/to/your/project
```

### 命令行索引模式

也可通过命令行参数手动构建索引（首次使用时会自动创建 Zilliz 数据集）：

```bash
./code-context-mcp -index /path/to/project
```

> **注意**：首次运行会创建名为 `code-context` 的集合。如果集合已存在，会先删除旧数据再重建索引。

## MCP 启动包装器 `start-mcp.sh`

`start-mcp.sh` 是一个启动包装脚本，自动从 `opencode.json` 中读取环境变量并注入到 MCP 进程，**解决 OpenCode 不自动注入 env 配置的问题**。

### 适用场景

| 场景 | 说明 |
|------|------|
| **单个项目** | 将 `start-mcp.sh` 和 `code_context_mcp` 复制到项目根目录 |
| **多个项目共用** | 将 `start-mcp.sh` 放在公共路径，各项目通过 `MCP_BINARY` 指定二进制路径 |
| **OpenCode 用户** | 必须使用包装器，因为 OpenCode 的 `mcp.code-context.env` 不会被自动注入 |
| **其他工具** | 如果工具本身支持 `command.env` 注入，则不需要包装器 |

### 快速使用

```bash
# 1. 将包装器和二进制复制到项目目录
cp /path/to/code-context-mcp/start-mcp.sh    ./start-mcp.sh
cp /path/to/code-context-mcp/code_context_mcp ./code_context_mcp

# 2. 查看版本（验证 env 注入是否正常）
./start-mcp.sh -version

# 3. 构建索引
./start-mcp.sh -index /path/to/your/project

# 4. 启动 MCP 服务器
./start-mcp.sh
```

> 提示：也可以使用软链代替复制 —— `ln -s` 一次后，重新 `make build` 会自动生效。

### 多项目共享配置

如果你有多个项目共用一个 MCP 服务，可以将包装器和二进制放在公共目录：

```bash
# 放在 ~/bin 或 /opt/code-context 等公共位置
cp code_context_mcp /opt/code-context/
cp start-mcp.sh     /opt/code-context/
```

然后在每个项目目录创建 `opencode.json`，`command` 指向公共路径：

```json
{
  "mcp": {
    "code-context": {
      "type": "local",
      "command": ["/opt/code-context/start-mcp.sh"],
      "env": {
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_api_token_here",
        "COLLECTION_NAME": "my_project",
        "AUTO_INDEX": "true",
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

> 每个项目的 `COLLECTION_NAME` 应不同，以避免向量数据互相干扰。

### 环境变量覆盖

包装器支持以下环境变量自定义行为：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `MCP_BINARY` | 脚本同目录下的 `code_context_mcp` | 指定 MCP 二进制路径 |
| `CONFIG_PATH` | 自动向上搜索 `opencode.json` | 指定配置文件路径 |

```bash
# 示例：手动指定配置文件和二进制
MCP_BINARY=/opt/code-context/code_context_mcp \
  CONFIG_PATH=/home/user/projects/my-app/opencode.json \
  ./start-mcp.sh -index /home/user/projects/my-app
```

### 搜索 `opencode.json` 的优先级

1. `CONFIG_PATH` 环境变量指定路径
2. 当前工作目录下的 `opencode.json`
3. 逐级向上搜索父目录的 `opencode.json`
4. `$HOME/.config/opencode/opencode.json`（用户级配置）

## AI 编程工具 MCP 配置

### CodeArts (华为云码道)

在 CodeArts 的 MCP 配置中添加：

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "args": [],
      "env": {
        "OLLAMA_URL": "http://localhost:11434",
        "OLLAMA_EMBED_MODEL": "nomic-embed-text:latest",
        "EMBEDDING_DIM": "768",
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_api_token_here",
        "COLLECTION_NAME": "code-context",
        "AUTO_INDEX": "true",
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

### OpenCode

> ⚠️ **重要**：OpenCode 目前**不会自动注入** `opencode.json` 中 `mcp.code-context.env` 里配置的环境变量，因此必须使用 `start-mcp.sh` 包装器来启动。

**项目级配置** (`opencode.json` 放在项目根目录)：

```json
{
  "mcp": {
    "code-context": {
      "type": "local",
      "command": ["./start-mcp.sh"],
      "env": {
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_api_token_here",
        "COLLECTION_NAME": "code-context",
        "AUTO_INDEX": "true",
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

**用户级配置** (`~/.config/opencode/opencode.json`)：

```json
{
  "mcp": {
    "code-context": {
      "type": "local",
      "command": ["/opt/code-context/start-mcp.sh"],
      "env": {
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_api_token_here",
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

### Cursor / Windsurf / Claude Desktop

这些工具支持直接在 MCP 配置中设置 `env`，无需包装器：

**项目级配置** (`.mcp.json` 放在项目根目录)：

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "args": [],
      "env": {
        "OLLAMA_URL": "http://localhost:11434",
        "OLLAMA_EMBED_MODEL": "nomic-embed-text:latest",
        "EMBEDDING_DIM": "768",
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_api_token_here",
        "COLLECTION_NAME": "code-context",
        "AUTO_INDEX": "true",
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

**用户级配置** (`~/.config/opencode/mcp.json` 或 `~/.cursor/mcp.json`)：

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_api_token_here",
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

> 用户级配置中，未列出的环境变量使用默认值。Ollama 默认 `localhost:11434`，AUTO_INDEX 默认 `true`。

### GitHub Copilot (VS Code)

在 VS Code 的 `settings.json` 中配置：

```json
{
  "mcp": {
    "servers": {
      "code-context": {
        "command": "/path/to/code-context-mcp",
        "args": [],
        "env": {
          "OLLAMA_URL": "http://localhost:11434",
          "OLLAMA_EMBED_MODEL": "nomic-embed-text:latest",
          "EMBEDDING_DIM": "768",
          "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
          "ZILLIZ_TOKEN": "your_api_token_here",
          "COLLECTION_NAME": "code-context",
          "AUTO_INDEX": "true",
          "PROJECT_PATH": "/path/to/your/project"
        }
      }
    }
  }
}
```

> Copilot MCP 支持需要 VS Code 1.99+ 并启用 `github.copilot.chat.mcp` 设置。

### 使用 .env 文件简化配置

如果可执行文件同目录下有 `.env` 文件，服务会自动加载，无需在 MCP 配置中重复所有环境变量。此时 MCP 配置可简化为：

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp"
    }
  }
}
```

## 工具使用指南

### code_search — 语义搜索

用自然语言描述搜索意图，返回语义最匹配的代码片段。

```
code_search(query="配置加载相关逻辑", top_k=5)
```

### symbol_search — 精确符号搜索

按符号名查找定义和引用，支持驼峰/下划线自动转换。

```
# 查找所有出现
symbol_search(query="LoadConfig")

# 仅查找定义
symbol_search(query="LoadConfig", search_type="definition")

# 仅查找引用
symbol_search(query="load_config", search_type="reference")
```

### impact_analysis — 影响范围分析

分析符号被删除/重命名/修改签名后的影响范围。

```
# 删除影响分析
impact_analysis(symbol="LoadConfig", action="delete")

# 重命名影响分析
impact_analysis(symbol="LoadConfig", action="rename", new_name="ReadConfig")

# 修改签名影响分析
impact_analysis(symbol="CodeChunk", action="modify")
```

### file_context — 文件内容/摘要

```
# 完整代码（默认）
file_context(file_path="config.go")

# 结构摘要（节省上下文窗口）
file_context(file_path="config.go", mode="summary")
```

摘要返回示例：
```json
{
  "file": "config.go",
  "lines": 102,
  "language": "go",
  "imports": ["os", "strconv", "strings"],
  "functions": [
    {"name": "LoadConfig", "line_start": 31, "line_end": 48},
    {"name": "getEnv", "line_start": 50, "line_end": 55}
  ],
  "types": [
    {"name": "Config", "kind": "struct", "line": 10}
  ]
}
```

### index_project — 手动索引

```
index_project(path="/path/to/project")
```

启用 `AUTO_INDEX=true` 后通常无需手动调用。

## 支持的语言切分策略

| 语言 | 扩展名 | 切分边界 |
|------|--------|---------|
| Go | `.go` | `func`, `type struct`, `type interface`, `var`, `const` |
| Vue | `.vue` | `<template>`, `<script>`, `<style>`，script 内按 JS/TS 切分 |
| JavaScript | `.js` | `function`, `class`, `export`, `const` |
| TypeScript | `.ts` | 同 JavaScript |
| Markdown | `.md` | 标题 `#`/`##`/`###` |
| Python | `.py` | `def`, `class` |

## 项目文档

| 文件 | 说明 |
|------|------|
| [SPEC.md](docs/SPEC.md) | 产品背景与需求规格 |
| [DESIGN.md](docs/DESIGN.md) | 实现方案（架构、详细设计、API） |
| [TASKS.md](docs/TASKS.md) | 实施方案（任务记录、文件清单） |

## 开发与构建

### 使用 Makefile

项目包含一个 Makefile 简化常见操作：

```bash
# 查看所有可用命令
make help

# 构建二进制文件
make build

# 运行测试
make test

# 运行代码检查
make lint

# 格式化代码
make fmt

# 构建所有平台
make build-all

# 创建 Docker 镜像
make docker-build

# 运行 Docker 容器
make docker-run
```

### Docker 使用

```bash
# 构建镜像
docker build -t code-context-mcp .

# 运行容器
docker run --rm -it \
  -e OLLAMA_URL=http://host.docker.internal:11434 \
  -e ZILLIZ_URI=https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com \
  -e ZILLIZ_TOKEN=your_api_token_here \
  -e PROJECT_PATH=/app/project \
  -v $(pwd):/app/project \
  code-context-mcp
```

> **注意**：Docker 容器需要访问宿主机上的 Ollama 服务，使用 `host.docker.internal` 作为主机地址。

## CI/CD 与发布

### GitHub Actions 工作流

项目配置了以下 GitHub Actions 工作流：

1. **测试工作流** (`test.yml`)：
   - 在 push 和 pull request 时运行
   - 支持 Go 1.25.x 和 1.26.x
   - 运行 `go vet`、`staticcheck` 和单元测试
   - 上传覆盖率报告到 Codecov

2. **构建工作流** (`build.yml`)：
   - 在 push 和 tag 时运行
   - 为 Linux、macOS、Windows 构建二进制文件
   - 构建多架构 Docker 镜像（linux/amd64, linux/arm64）

3. **发布工作流** (`release.yml`)：
   - 在推送 tag 时运行（如 `v1.0.0`）
   - 使用 GoReleaser 创建 GitHub Release
   - 发布多平台二进制包和 Docker 镜像

### 发布新版本

```bash
# 1. 更新版本号（遵循语义化版本）
git tag -a v1.0.0 -m "Release v1.0.0"

# 2. 推送标签触发发布
git push origin v1.0.0
```

发布后可在 [GitHub Releases](https://github.com/handy-h/code-context-mcp/releases) 页面下载预编译的二进制文件。

### 预编译二进制文件

每个发布版本包含以下平台的二进制文件：
- `code-context-mcp_linux_x86_64.tar.gz` (Linux amd64)
- `code-context-mcp_linux_arm64.tar.gz` (Linux arm64)
- `code-context-mcp_darwin_x86_64.tar.gz` (macOS Intel)
- `code-context-mcp_darwin_arm64.tar.gz` (macOS Apple Silicon)
- `code-context-mcp_windows_x86_64.zip` (Windows amd64)

### Docker 镜像

发布时会自动构建并推送 Docker 镜像到 GitHub Container Registry：
- `ghcr.io/handy-h/code-context-mcp:latest` (最新版本)
- `ghcr.io/handy-h/code-context-mcp:v1.0.0` (特定版本)

## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## License

[MIT](LICENSE)
