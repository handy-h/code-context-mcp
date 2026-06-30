# code-context-mcp

为 AI 编码助手提供代码上下文的 MCP 服务，通过语义搜索、符号查找、影响分析等能力，减少上下文调用，替代 Grep/Glob/Read 的组合使用。

## 功能概览

| 工具              | 说明                 | 典型场景                                 |
| ----------------- | -------------------- | ---------------------------------------- |
| `code_search`     | 语义搜索代码片段     | "处理用户认证的逻辑"、"配置加载相关代码" |
| `symbol_search`   | 精确符号搜索         | 查找 `LoadConfig` 函数的所有引用         |
| `impact_analysis` | 影响范围分析         | 删除/重命名某函数后影响哪些文件          |
| `file_context`    | 文件内容或结构摘要   | 先看摘要定位函数，再精确读取             |
| `token_stats`     | 查看 token 节省统计  | 了解工具调用节省了多少上下文 token       |
| `index_project`   | 手动索引构建         | 首次使用或代码大变更后                   |

## 核心特性

- **自动索引**：MCP 启动时自动检测索引状态，搜索时发现过期自动后台增量更新，无需手动调用 `index_project`
- **本地向量存储**：默认使用本地 JSONL 文件保存向量，Zilliz Cloud 作为可选后端
- **按代码结构切分**：按函数/结构体/组件等语法边界切分，而非固定字符窗口，搜索结果语义完整
- **精确符号搜索**：基于倒排索引，支持驼峰/下划线模糊匹配，结果按文件分组含上下文摘要
- **影响范围分析**：一次调用完成 delete/rename/modify 的影响分析，返回修改建议
- **文件结构摘要**：`file_context(mode="summary")` 返回函数列表、类型列表、导入依赖，节省上下文窗口

## 快速开始

### 1. 安装前置依赖

#### 嵌入模型服务（选择一种）

**选项 A: Ollama（本地嵌入模型服务）**

```bash
# 安装 Ollama（参考 https://ollama.com）
# 安装后拉取嵌入模型
ollama pull nomic-embed-text:latest
```

**选项 B: OpenAI 兼容 API（云端服务）**

- OpenAI API：注册并获取 API 密钥
- 兼容 OpenAI API 的服务：如 Azure OpenAI、OpenRouter、LiteLLM 等
- 需要配置 `OPENAI_API_KEY` 环境变量

**向量存储** — 默认本地 JSONL，可选 Zilliz Cloud

- 本地 JSONL：默认配置，无需远程向量数据库，适合个人项目和中小型代码库
- Zilliz Cloud：设置 `VECTOR_STORE=zilliz` 后使用，适合团队共享或更大规模向量检索
- 注册 [Zilliz Cloud](https://cloud.zilliz.com/) 获取 URI 和 API Token
- 免费套餐即可满足个人项目需求

### 2. 克隆并构建

```bash
git clone https://github.com/handy-h/code-context-mcp.git
cd code-context-mcp
make build
```

`make build` 会将可执行文件构建到项目根目录的 `code-context-mcp/code-context-mcp` 目录。本地 JSONL 模式索引后也会把向量文件写到可执行文件同目录，便于把整个 `code-context-mcp/` 复制到其他项目中配置 MCP。

对于 Windows 用户，可以使用 `.\build.ps1 build` 命令，效果相同。

### 3. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，填入项目路径。默认使用本地 JSONL：

```env
VECTOR_STORE=local
PROJECT_PATH=/path/to/your/project
```

如需继续使用 Zilliz：

```env
VECTOR_STORE=zilliz
ZILLIZ_URI=https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com
ZILLIZ_TOKEN=your_api_token_here
PROJECT_PATH=/path/to/your/project
```

### 4. 初始化索引

首次使用时，需要初始化向量索引。有两种方式：

**方式一：自动初始化（推荐）**

1. 确保 `AUTO_INDEX=true`（默认值）
2. 启动 MCP 服务（通过 AI 编程工具配置）
3. 服务启动时会自动检测并创建本地 JSONL 文件或 Zilliz 数据集

**方式二：手动初始化**

```bash
# 构建索引，自动写入本地 JSONL 或 Zilliz
./code-context-mcp/code-context-mcp -index /path/to/your/project
```

**本地 JSONL 验证**：

索引完成后查看 `code-context-mcp/code_context.jsonl` 是否生成。

**Zilliz 数据集验证**：

1. 登录 [Zilliz Cloud 控制台](https://cloud.zilliz.com/)
2. 进入你的集群
3. 在 "Collections" 页面查看是否创建了名为 `code-context` 的集合

### 5. 配置 AI 编程工具

参考下方 [AI 编程工具 MCP 配置](#ai-编程工具-mcp-配置) 章节，将 `code-context-mcp` 添加到你的 AI 编程工具中。

## 配置

### 环境变量

通过 `.env` 文件（与可执行文件同目录）或系统环境变量配置：

#### 嵌入模型配置

| 环境变量             | 默认值   | 说明                                |
| -------------------- | -------- | ----------------------------------- |
| `EMBEDDING_PROVIDER` | `ollama` | 嵌入模型提供商 (`ollama`, `openai`) |
| `EMBEDDING_DIM`      | `768`    | 向量维度（需与模型匹配）            |

#### Ollama 配置（当 `EMBEDDING_PROVIDER=ollama` 时使用）

| 环境变量             | 默认值                    | 说明            |
| -------------------- | ------------------------- | --------------- |
| `OLLAMA_URL`         | `http://localhost:11434`  | Ollama 服务地址 |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text:latest` | 嵌入模型名称    |

#### OpenAI 兼容 API 配置（当 `EMBEDDING_PROVIDER=openai` 时使用）

| 环境变量             | 默认值                      | 说明         |
| -------------------- | --------------------------- | ------------ |
| `OPENAI_BASE_URL`    | `https://api.openai.com/v1` | API 基础地址 |
| `OPENAI_EMBED_MODEL` | `text-embedding-ada-002`    | 嵌入模型名称 |
| `OPENAI_API_KEY`     | （必填）                    | API 密钥     |

#### 向量数据库配置

| 环境变量          | 默认值         | 说明                                |
| ----------------- | -------------- | ----------------------------------- |
| `VECTOR_STORE`    | `local`        | 向量存储后端：`local`/`local-jsonl`/`jsonl` 或 `zilliz` |
| `VECTOR_STORE_PATH` | （自动）     | 本地 JSONL 文件路径，默认写入 `code-context-mcp/{COLLECTION_NAME}.jsonl` |
| `ZILLIZ_URI`      | （可选）       | Zilliz Cloud URI，仅 `VECTOR_STORE=zilliz` 时需要 |
| `ZILLIZ_TOKEN`    | （可选）       | Zilliz Cloud API Token，仅 `VECTOR_STORE=zilliz` 时需要 |
| `COLLECTION_NAME` | `code-context` | Milvus 集合名，首次使用时会自动创建 |

#### 索引配置

| 环境变量           | 默认值                     | 说明                                                                   |
| ------------------ | -------------------------- | ---------------------------------------------------------------------- |
| `SCAN_EXTENSIONS`  | `.go,.vue,.js,.ts,.py,.md,.rs` | 扫描的文件扩展名                                                       |
| `CHUNK_SIZE`       | `800`                      | 降级切分时的块大小（rune）                                             |
| `MAX_CHUNK_SIZE`   | `1500`                     | 结构切分后超长块的最大 rune 数                                         |
| `AUTO_INDEX`       | `true`                     | 是否启用自动索引                                                       |
| `PROJECT_PATH`     | （空）                     | MCP 模式下自动索引的项目路径                                           |
| `INDEX_STATE_PATH` | （自动）                   | 索引状态文件路径，默认 `{exe_dir}/index-state.json` |

#### Token 节省统计配置

| 环境变量                              | 默认值     | 说明                                           |
| ------------------------------------- | ---------- | ---------------------------------------------- |
| `TOKEN_STATS_ENABLED`                 | `false`    | 是否启用 token 节省统计                         |
| `TOKEN_STATS_PATH`                    | （自动）   | 统计文件路径，默认 `{exe_dir}/token-stats.json` |
| `TOKEN_STATS_CHARS_PER_TOKEN`         | `4.0`      | ASCII 字符/token 转换系数                      |
| `TOKEN_STATS_CODE_SEARCH_BASELINE`    | `2000`     | `code_search` 每个文件的基线 token 数            |
| `TOKEN_STATS_FILE_CONTEXT_BASELINE`   | `3000`     | `file_context` summary 模式的基线 token 数      |
| `TOKEN_STATS_SYMBOL_SEARCH_BASELINE`  | `8000`     | `symbol_search` 的基线 token 数                  |
| `TOKEN_STATS_IMPACT_ANALYSIS_BASELINE`| `12000`    | `impact_analysis` 的基线 token 数                |
| `TOKEN_STATS_RETENTION_DAYS`          | `90`       | 统计数据保留天数                               |

### 配置选择指南

根据你的使用场景选择合适的配置方案：

| 场景                  | 推荐配置          | 优点                        | 缺点                    | 适用场景                         |
| --------------------- | ----------------- | --------------------------- | ----------------------- | -------------------------------- |
| **本地开发/测试**     | Ollama + 本地模型 | 免费、离线可用、隐私安全    | 需要本地资源、性能一般  | 个人项目、开发环境、隐私敏感项目 |
| **生产环境/团队协作** | OpenAI API        | 高性能、稳定、易于部署      | 需要API费用、网络依赖   | 企业项目、团队协作、生产环境     |
| **Google Cloud 用户** | Google Gemini     | Google生态集成、稳定可靠    | Google特定、需要API密钥 | Google Cloud用户、Gemini生态     |
| **Azure 企业用户**    | Azure OpenAI      | 企业级安全、合规性、SLA保证 | Azure特定、配置复杂     | Azure云用户、企业合规需求        |
| **多模型支持**        | OpenRouter        | 支持多种模型、灵活选择      | 第三方服务、可能有延迟  | 需要切换不同模型的场景           |
| **自托管/代理**       | LiteLLM + 自托管  | 完全控制、可自托管模型      | 需要维护、配置复杂      | 有自托管需求的团队               |

### 配置文件模板

我们提供了多个配置文件模板，根据你的使用场景选择合适的模板：

#### 快速开始

```bash
# 使用默认模板（Ollama）
cp .env.example .env

# 或者使用特定场景的模板
cp configs/examples/.env.ollama.example .env        # Ollama 本地部署
cp configs/examples/.env.openai.example .env         # OpenAI 官方 API
cp configs/examples/.env.gemini.example .env         # Google Gemini API
cp configs/examples/.env.openai-compatible.example .env  # 兼容 OpenAI API 的服务
```

#### 配置文件模板说明

项目提供了以下配置文件模板：

| 模板文件                                         | 适用场景     | 说明                                     |
| ------------------------------------------------ | ------------ | ---------------------------------------- |
| `.env.example`                                   | 通用模板     | 包含所有配置项，注释详细                 |
| `configs/examples/.env.ollama.example`            | 本地开发     | 使用 Ollama 本地嵌入模型服务             |
| `configs/examples/.env.openai.example`            | 生产环境     | 使用 OpenAI 官方 API                     |
| `configs/examples/.env.gemini.example`            | Google Cloud | 使用 Google Gemini API                   |
| `configs/examples/.env.openai-compatible.example` | 兼容服务     | 使用 Azure OpenAI、OpenRouter 等兼容服务 |

#### 使用步骤

1. **选择模板**: 根据你的场景选择合适的模板
2. **复制配置**: 将模板复制为 `.env` 文件
3. **编辑配置**: 填入你的实际配置值
4. **运行服务**: 启动 MCP 服务

```env
# ============================================================
# 嵌入模型配置
# ============================================================

# 嵌入模型提供商 (支持: ollama, openai)
EMBEDDING_PROVIDER=ollama

# 嵌入向量维度
# Ollama nomic-embed-text: 768
# OpenAI text-embedding-ada-002: 1536
EMBEDDING_DIM=768

# ============================================================
# Ollama 配置 (当 EMBEDDING_PROVIDER=ollama 时使用)
# ============================================================

# Ollama 服务地址
OLLAMA_URL=http://localhost:11434

# Ollama 嵌入模型名称
OLLAMA_EMBED_MODEL=nomic-embed-text:latest

# ============================================================
# OpenAI 兼容 API 配置 (当 EMBEDDING_PROVIDER=openai 时使用)
# ============================================================

# OpenAI 兼容 API 基础地址
# OPENAI_BASE_URL=https://api.openai.com/v1

# OpenAI 嵌入模型名称
# OPENAI_EMBED_MODEL=text-embedding-ada-002

# OpenAI API 密钥
# OPENAI_API_KEY=your_openai_api_key_here

# ============================================================
# 向量存储配置
# ============================================================

VECTOR_STORE=local
# VECTOR_STORE_PATH=/path/to/code-context-mcp/code_context.jsonl

# 仅 VECTOR_STORE=zilliz 时需要：
ZILLIZ_URI=https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com
ZILLIZ_TOKEN=your_zilliz_token_here
COLLECTION_NAME=code_context

# ============================================================
# 索引配置
# ============================================================

SCAN_EXTENSIONS=.go,.vue,.js,.ts,.py,.md,.rs
CHUNK_SIZE=800
MAX_CHUNK_SIZE=1500
AUTO_INDEX=true
PROJECT_PATH=/path/to/your/project
```

### 配置说明

#### 使用 Ollama 嵌入模型（默认）

这是默认配置，适合本地开发环境：

```env
EMBEDDING_PROVIDER=ollama
EMBEDDING_DIM=768
OLLAMA_URL=http://localhost:11434
OLLAMA_EMBED_MODEL=nomic-embed-text:latest
```

确保 Ollama 服务正在运行：

```bash
ollama serve
```

#### 使用 OpenAI 兼容 API

适合云环境或需要更高性能的场景：

```env
EMBEDDING_PROVIDER=openai
EMBEDDING_DIM=1536  # 根据模型调整
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_EMBED_MODEL=text-embedding-ada-002
OPENAI_API_KEY=your_api_key_here
```

支持的模型和维度：

- `text-embedding-ada-002`: 1536 维
- `text-embedding-3-small`: 1536 维
- `text-embedding-3-large`: 3072 维

#### 使用其他兼容 OpenAI API 的服务

支持任何兼容 OpenAI Embeddings API 的服务：

```env
EMBEDDING_PROVIDER=openai
EMBEDDING_DIM=1536  # 根据实际模型调整
OPENAI_BASE_URL=https://your-api-endpoint.com/v1
OPENAI_EMBED_MODEL=your-model-name
OPENAI_API_KEY=your_api_key_here
```

兼容的服务包括：

- Google Gemini
- Azure OpenAI
- OpenRouter
- LiteLLM
- 其他兼容 OpenAI API 的嵌入服务

#### 使用 Google Gemini API

适合 Google Cloud 用户或需要 Gemini 模型的场景：

```env
EMBEDDING_PROVIDER=gemini
EMBEDDING_DIM=768  # Gemini embedding-001 为 768 维
GEMINI_BASE_URL=https://generativelanguage.googleapis.com/v1beta
GEMINI_EMBED_MODEL=embedding-001
GEMINI_API_KEY=your_gemini_api_key_here
```

获取 Gemini API 密钥：

1. 访问 [Google AI Studio](https://makersuite.google.com/app/apikey)
2. 创建 API 密钥
3. 复制密钥到配置中

### MCP 配置文件写法

#### OpenCode MCP 配置

在 OpenCode 或 Claude Desktop 的 MCP 配置文件中，可以这样配置：

**使用 Ollama（本地开发）：**

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "EMBEDDING_PROVIDER": "ollama",
        "OLLAMA_URL": "http://localhost:11434",
        "OLLAMA_EMBED_MODEL": "nomic-embed-text:latest",
        "EMBEDDING_DIM": "768",
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_zilliz_token_here",
        "COLLECTION_NAME": "code_context",
        "PROJECT_PATH": "/path/to/your/project",
        "SCAN_EXTENSIONS": ".go,.vue,.js,.ts,.py,.md,.rs",
        "CHUNK_SIZE": "800",
        "MAX_CHUNK_SIZE": "1500",
        "AUTO_INDEX": "true"
      }
    }
  }
}
```

**使用 OpenAI API：**

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "EMBEDDING_PROVIDER": "openai",
        "EMBEDDING_PROVIDER": "openai",
        "OPENAI_BASE_URL": "https://api.openai.com/v1",
        "OPENAI_EMBED_MODEL": "text-embedding-ada-002",
        "OPENAI_API_KEY": "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        "EMBEDDING_DIM": "1536",
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_zilliz_token_here",
        "COLLECTION_NAME": "code_context",
        "PROJECT_PATH": "/path/to/your/project",
        "SCAN_EXTENSIONS": ".go,.vue,.js,.ts,.py,.md,.rs",
        "CHUNK_SIZE": "800",
        "MAX_CHUNK_SIZE": "1500",
        "AUTO_INDEX": "true"
      }
    }
  }
}
```

**使用 Azure OpenAI：**

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "EMBEDDING_PROVIDER": "openai",
        "OPENAI_BASE_URL": "https://your-resource.openai.azure.com/openai/deployments/text-embedding-ada-002",
        "OPENAI_EMBED_MODEL": "text-embedding-ada-002",
        "OPENAI_API_KEY": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        "EMBEDDING_DIM": "1536",
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_zilliz_token_here",
        "COLLECTION_NAME": "code_context",
        "PROJECT_PATH": "/path/to/your/project",
        "SCAN_EXTENSIONS": ".go,.vue,.js,.ts,.py,.md,.rs",
        "CHUNK_SIZE": "800",
        "MAX_CHUNK_SIZE": "1500",
        "AUTO_INDEX": "true"
      }
    }
  }
}
```

**使用 Google Gemini：**

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "EMBEDDING_PROVIDER": "gemini",
        "EMBEDDING_PROVIDER": "gemini",
        "GEMINI_BASE_URL": "https://generativelanguage.googleapis.com/v1beta",
        "GEMINI_EMBED_MODEL": "embedding-001",
        "GEMINI_API_KEY": "AIzaSyxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        "EMBEDDING_DIM": "768",
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_zilliz_token_here",
        "COLLECTION_NAME": "code_context",
        "PROJECT_PATH": "/path/to/your/project",
        "SCAN_EXTENSIONS": ".go,.vue,.js,.ts,.py,.md,.rs",
        "CHUNK_SIZE": "800",
        "MAX_CHUNK_SIZE": "1500",
        "AUTO_INDEX": "true"
      }
    }
  }
}
```

**使用 OpenRouter：**

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "EMBEDDING_PROVIDER": "openai",
        "OPENAI_BASE_URL": "https://openrouter.ai/api/v1",
        "OPENAI_EMBED_MODEL": "openai/text-embedding-ada-002",
        "OPENAI_API_KEY": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        "EMBEDDING_DIM": "1536",
        "ZILLIZ_URI": "https://your-instance.serverless.gcp-us-west1.cloud.zilliz.com",
        "ZILLIZ_TOKEN": "your_zilliz_token_here",
        "COLLECTION_NAME": "code_context",
        "PROJECT_PATH": "/path/to/your/project",
        "SCAN_EXTENSIONS": ".go,.vue,.js,.ts,.py,.md,.rs",
        "CHUNK_SIZE": "800",
        "MAX_CHUNK_SIZE": "1500",
        "AUTO_INDEX": "true"
      }
    }
  }
}
```

#### 简化配置

如果使用 `.env` 文件，可以简化 MCP 配置：

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

如果可执行文件同目录下有 `.env` 文件，服务会自动加载环境变量。

### 命令行索引模式

也可通过命令行参数手动构建索引：

```bash
./code-context-mcp -index /path/to/project
```

> **注意**：首次运行会创建名为 `code-context` 的集合。如果集合已存在，会先删除旧数据再重建索引。

### 环境变量覆盖

如果使用 `.env` 文件，可以直接在 MCP 配置中指定 `PROJECT_PATH`，其他环境变量从 `.env` 加载：

```json
{
  "mcpServers": {
    "code-context": {
      "command": "/path/to/code-context-mcp",
      "env": {
        "PROJECT_PATH": "/path/to/your/project"
      }
    }
  }
}
```

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

**项目级配置** (`opencode.json` 放在项目根目录)：

```json
{
  "mcp": {
    "code-context": {
      "type": "local",
      "command": ["./code-context-mcp"],
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
      "command": ["/path/to/code-context-mcp"],
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
    { "name": "LoadConfig", "line_start": 31, "line_end": 48 },
    { "name": "getEnv", "line_start": 50, "line_end": 55 }
  ],
  "types": [{ "name": "Config", "kind": "struct", "line": 10 }]
}
```

### index_project — 手动索引

```
index_project(path="/path/to/project")
```

启用 `AUTO_INDEX=true` 后通常无需手动调用。

### token_stats — Token 节省统计

查看 MCP 工具使用期间累计节省的 token 统计信息，按日维度聚合。基于基线对照法估算——将每次工具调用的实际输出 token 数与不使用本工具时假设需要的 token 数（基线）对比，计算节省量。

```
token_stats()
```

无参数。输出示例：

```
=== Token 节省统计 ===
统计周期: 2026-06-01 ~ 2026-06-29
总调用次数: 142
总输出 token: 89,432
总节省 token: 287,568 (估算)

按工具分组:
工具              调用次数  输出token   节省token   平均耗时
code_search       68       42,180     156,420    320ms
file_context      45       28,500     18,200     5ms
symbol_search     18       12,400     89,200     15ms
impact_analysis   11       6,352      23,748     45ms

近 7 天趋势:
日期        调用次数  节省token
06-23       12       24,100
06-24       18       36,800
06-25       22       41,200
06-26       15       28,500
06-27       20       38,900
06-28       25       52,300
06-29       30       65,768

说明:
- 节省量基于基线对照法估算，仅供参考
- 可通过 TOKEN_STATS_*_BASELINE 调整基线
- 统计保留 90 天，可通过 TOKEN_STATS_RETENTION_DAYS 调整
```

#### 基线说明

每个工具的节省量基准如下：

| 工具 | 基线估算方式 |
|------|-------------|
| `code_search` | `top_k * 2000`（top_k 默认 5，上限 10） |
| `file_context` | `mode="summary"` 时 3000，`mode="full"` 时 0（无节省） |
| `symbol_search` | 8000 |
| `impact_analysis` | 12000 |
| `index_project` / `token_stats` | 不参与统计 |

节省量 = `max(0, 基线 - 实际输出 token)`，即只有输出少于基线时才视为有节省。

#### 启用方法

默认关闭。在 `.env` 或环境变量中设置以下配置即可启用：

```env
TOKEN_STATS_ENABLED=true
```

统计文件（`token-stats.json`）默认与可执行文件同目录，随服务启动自动创建或续写。可通过 `TOKEN_STATS_PATH` 自定义路径。

> **注意**：`token_stats` 自身的调用不计入统计，`index_project` 的索引过程也不计入，避免循环和干扰。

## 支持的语言切分策略

| 语言       | 扩展名 | 切分边界                                                    |
| ---------- | ------ | ----------------------------------------------------------- |
| Go         | `.go`  | `func`, `type struct`, `type interface`, `var`, `const`     |
| Vue        | `.vue` | `<template>`, `<script>`, `<style>`，script 内按 JS/TS 切分 |
| JavaScript | `.js`  | `function`, `class`, `export`, `const`                      |
| TypeScript | `.ts`  | 同 JavaScript                                               |
| Markdown   | `.md`  | 标题 `#`/`##`/`###`                                         |
| Python     | `.py`  | `def`, `class`                                              |
| Rust       | `.rs`  | `fn`, `struct`, `enum`, `trait`, `impl`, `mod`, `type`, `const`, `static`, `macro_rules!` |

## 项目文档

| 文件                        | 说明                            |
| --------------------------- | ------------------------------- |
| [SPEC.md](docs/SPEC.md)     | 产品背景与需求规格              |
| [DESIGN.md](docs/DESIGN.md) | 实现方案（架构、详细设计、API） |
| [TASKS.md](docs/TASKS.md)   | 实施方案（任务记录、文件清单）  |

## 开发与构建

### 使用 Makefile（Linux/macOS）

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

# 清理构建产物
make clean
```

### 使用 PowerShell 脚本（Windows）

对于 Windows 用户，项目提供了 `build.ps1` PowerShell 脚本，提供与 Makefile 相同的功能：

```powershell
# 查看所有可用命令
.\build.ps1 help

# 构建二进制文件到 code-context-mcp/ 目录
.\build.ps1 build

# 运行测试
.\build.ps1 test

# 清理构建产物
.\build.ps1 clean

# 运行代码检查
.\build.ps1 lint

# 格式化代码
.\build.ps1 fmt

# 显示版本信息
.\build.ps1 version

# 构建并运行开发模式
.\build.ps1 dev

# 运行测试并生成覆盖率报告
.\build.ps1 test-coverage

# 安装到 GOPATH/bin
.\build.ps1 install
```

详细的使用说明请参考 [BUILD_WINDOWS.md](BUILD_WINDOWS.md)。



## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## License

[MIT](LICENSE)
