# MCP 每次启动都重建索引的问题分析与修复

作者：你
日期：2026-05-23

## 1) Assess（评估）

- **背景与范围**
  - code-context-mcp 在 MCP 服务器模式下启动时，会通过 `IndexManager.CheckAndAutoIndex` 自动检测索引状态。
  - 用户反馈：Claude Code 每次使用该 MCP 时都会触发全量重建索引，严重影响启动性能（大型项目全量重建可能耗时数分钟），且部分情况下报告重建失败。

- **已知信息**
  - 索引状态通过指纹（fingerprint）判断是否需要重建：git 仓库用 `git rev-parse HEAD` 的 commit hash，非 git 仓库用文件 mtime 的 SHA256 摘要。
  - 状态文件保存在 `{PROJECT_PATH}/.code-context-index-state.json`。
  - 全量重建（`fullBuild`）会先删除整个向量库再重建，而增量更新（`incrementalUpdate`）仅重建有 mtime 变化的文件。

- **观察与证据**
  - VoidText 项目：状态文件指纹 `90fe1f67...`，当前 git HEAD `2801e9b0...`，不一致 → 每次启动触发全量重建。
  - LabTrace 项目：`opencode.json` 中 `PROJECT_PATH=/home/gao/Builds/Labtrace`（小写 t），实际目录为 `/home/gao/Builds/LabTrace`（大写 T），Linux 路径区分大小写，导致每次启动都找不到状态文件 → 全量重建。

- **目标**
  - 修复 `computeMtimeFingerprint` 的确定性问题，保证相同文件集合每次计算出相同指纹。
  - 优化 `CheckAndAutoIndex` 策略：指纹不匹配时优先走增量更新，而非直接全量重建。
  - 记录 `PROJECT_PATH` 大小写配置问题，避免同类错误。

## 2) Route（路线）

- **根本原因分析**

  | 问题 | 根本原因 | 影响范围 |
  |------|----------|----------|
  | Bug 1: 非 git 项目每次指纹不同 | `computeMtimeFingerprint` 遍历 map，Go map 迭代顺序随机，导致 SHA256 每次不同 | 非 git 仓库项目 |
  | Bug 2: 有新提交就全量重建 | `CheckAndAutoIndex` 指纹不匹配时直接调 `fullBuild`，而非先尝试增量更新 | 所有 git 仓库项目 |
  | Bug 3: 配置 PROJECT_PATH 大小写错误 | `LabTrace/opencode.json` 写成 `Labtrace`，Linux 路径区分大小写，状态文件路径每次不同 | LabTrace 项目（配置问题） |

- **方案选择**
  - Bug 1：对 `computeMtimeFingerprint` 的 keys 排序后再写入哈希，保证确定性。
  - Bug 2：`CheckAndAutoIndex` 中指纹不匹配时改走 `incrementalUpdate`，失败才回退 `fullBuild`。
  - Bug 3：修正 `LabTrace/opencode.json` 中 `PROJECT_PATH` 的大小写。

## 3) Execute & Verify（执行与验证）

### Bug 1 修复：`computeMtimeFingerprint` 排序

**文件**：`internal/indexer/index_state.go`

```go
// 修复前（map 迭代顺序随机，每次哈希不同）
func computeMtimeFingerprint(mtimes map[string]string) string {
    h := sha256.New()
    for path, mtime := range mtimes {
        h.Write([]byte(path + ":" + mtime + "\n"))
    }
    return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// 修复后（对 keys 排序，哈希确定）
func computeMtimeFingerprint(mtimes map[string]string) string {
    h := sha256.New()
    keys := make([]string, 0, len(mtimes))
    for k := range mtimes {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    for _, path := range keys {
        h.Write([]byte(path + ":" + mtimes[path] + "\n"))
    }
    return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
```

同时在 imports 中添加 `"sort"` 包。

### Bug 2 修复：指纹不匹配时走增量更新

**文件**：`internal/indexer/index_manager.go`

```go
// 修复前（直接全量重建）
log.Println("索引已过期（指纹不匹配），开始全量索引构建...")
return mgr.fullBuild(ctx)

// 修复后（先增量，失败再全量）
log.Println("索引已过期（指纹不匹配），开始增量更新...")
go mgr.rebuildInvertedIndex(ctx)
if err := mgr.incrementalUpdate(ctx); err != nil {
    log.Printf("增量更新失败（%v），回退到全量重建...", err)
    return mgr.fullBuild(ctx)
}
mgr.mu.Lock()
mgr.stale = false
mgr.mu.Unlock()
return nil
```

### Bug 3 修复：LabTrace PROJECT_PATH 配置（配置修正，非代码）

修改 `/home/gao/Builds/LabTrace/opencode.json`：

```json
// 修复前
"PROJECT_PATH": "/home/gao/Builds/Labtrace"

// 修复后
"PROJECT_PATH": "/home/gao/Builds/LabTrace"
```

### 验证

```bash
# 编译验证
cd /home/gao/Builds/code-context-mcp
go build ./...
go vet ./...

# 部署到各项目
make deploy DEPLOY_DIR=/home/gao/Builds/VoidText
make deploy DEPLOY_DIR=/home/gao/Builds/LabTrace
```

## 4) 影响分析

- Bug 1 修复后，相同文件集合的 mtime 指纹确定，非 git 项目不再每次重建。
- Bug 2 修复后，有新 commit 时仅重建有变化的文件（增量），显著减少重建时间。
  - 增量更新利用 `state.FileMtimes` 对比当前 mtime，只重新向量化变更文件。
  - 若增量失败（如首次部署向量文件丢失），自动回退全量重建，保证鲁棒性。
- 两个修复均向后兼容，不影响已有的索引状态文件格式。

## 5) 根因总结与预防

| 类别 | 教训 |
|------|------|
| Go map 确定性 | 需要对 map 内容产生稳定哈希时，必须先对 keys 排序 |
| 指纹策略 | git hash 只反映最后 commit，适合判断"是否需要更新"，但不应直接触发全量重建；应优先增量 |
| 路径配置 | Linux 路径区分大小写，`PROJECT_PATH` 配置需与实际目录名完全一致 |

## 6) 变更记录

- **v1.0**（2026-05-23）
  - 诊断并修复 `computeMtimeFingerprint` map 迭代顺序不确定性问题
  - 优化 `CheckAndAutoIndex` 策略：指纹不匹配时改走增量更新
  - 记录 LabTrace `PROJECT_PATH` 大小写配置错误
