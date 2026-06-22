# GitHub Actions CI/CD 配置

本项目使用 GitHub Actions 进行持续集成和持续部署。

## 工作流说明

### 1. CI (`ci.yml`)
- **触发条件**: 推送到 `main` 或 `develop` 分支，以及针对 `main` 分支的 Pull Request
- **功能**:
  - 在 Go 1.25、1.26 版本上运行测试
  - 执行代码检查（golangci-lint）
  - 运行单元测试并生成覆盖率报告
  - 构建二进制文件（注入版本信息）

### 2. Docker (`docker.yml`)
- **触发条件**: 推送到 `main` 分支或推送版本标签，以及 Pull Request
- **功能**:
  - 构建 Docker 镜像（支持 linux/amd64 和 linux/arm64）
  - 推送到 GitHub Container Registry（ghcr.io）
  - 自动标签管理（分支、PR、版本标签）

### 3. Release (`release.yml`)
- **触发条件**: 推送新的版本标签（格式：`v*`）
- **功能**:
  - 为多个平台构建二进制文件（linux/darwin/windows × amd64/arm64）
  - 自动生成 checksums 文件
  - 创建 GitHub Release 并上传所有二进制文件

## 本地开发命令

```bash
make help           # 查看所有可用命令
make build          # 构建项目
make test           # 运行测试
make test-coverage  # 生成测试覆盖率报告
make lint           # 代码检查
make fmt            # 代码格式化
make vet            # 运行静态检查
make clean          # 清理构建文件
make dev            # 本地开发运行（索引当前目录）
make build-all      # 构建所有平台的二进制文件
```

## 创建新版本

1. 更新代码并推送到 `main` 分支
2. 创建新的版本标签：
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
3. GitHub Actions 会自动构建并发布新版本
