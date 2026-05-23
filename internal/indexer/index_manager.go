package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/handy-h/code-context-mcp/internal/embedding"
	"github.com/handy-h/code-context-mcp/internal/search"
	"github.com/handy-h/code-context-mcp/pkg/structure"
)

// IndexManager 索引管理器
type IndexManager struct {
	cfg         config.Config
	stateStore  *IndexStateStore
	invIndex    *search.InvertedIndex
	mu          sync.RWMutex
	stale       bool
	updating    atomic.Bool
	projectPath string
}

// NewIndexManager 创建索引管理器
func NewIndexManager(cfg config.Config, projectPath string) *IndexManager {
	stateStore := NewIndexStateStore(projectPath, cfg.IndexStatePath)
	return &IndexManager{
		cfg:         cfg,
		stateStore:  stateStore,
		invIndex:    search.NewInvertedIndex(),
		projectPath: projectPath,
		stale:       true,
	}
}

// CheckAndAutoIndex 启动时检查索引状态并自动索引
func (mgr *IndexManager) CheckAndAutoIndex(ctx context.Context) error {
	if !mgr.cfg.AutoIndex {
		slog.Info("自动索引已禁用")
		return nil
	}

	// 尝试加载索引状态
	state, err := mgr.stateStore.Load()
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			return err
		}
		// 状态文件不存在或损坏，触发全量构建
		slog.Info("索引状态不存在或已损坏，开始全量构建")
		return mgr.fullBuild(ctx)
	}

	// 比对指纹
	currentFingerprint, _, fpErr := mgr.stateStore.GetCurrentFingerprint(mgr.projectPath, mgr.cfg.ScanExtensions)
	if fpErr != nil {
		slog.Warn("计算索引指纹失败，标记索引为过期", "err", fpErr)
		mgr.mu.Lock()
		mgr.stale = true
		mgr.mu.Unlock()
		return nil
	}

	if currentFingerprint == state.Fingerprint {
		slog.Info("索引状态有效，无需重建")
		mgr.mu.Lock()
		mgr.stale = false
		mgr.mu.Unlock()
		// 重建倒排索引（内存中，重启后丢失）
		go mgr.rebuildInvertedIndex(ctx)
		return nil
	}

	// 指纹不匹配（有新提交或文件变更），走增量更新而非全量重建
	// fullBuild 会删除整个向量库再重建，对活跃项目代价过高
	slog.Info("索引已过期（指纹不匹配），开始增量更新")
	go mgr.rebuildInvertedIndex(ctx)
	if err := mgr.incrementalUpdate(ctx); err != nil {
		slog.Warn("增量更新失败，回退到全量重建", "err", err)
		return mgr.fullBuild(ctx)
	}
	mgr.mu.Lock()
	mgr.stale = false
	mgr.mu.Unlock()
	return nil
}

// fullBuild 全量构建索引
func (mgr *IndexManager) fullBuild(ctx context.Context) error {
	vdb, err := search.NewVectorDB(ctx, mgr.cfg)
	if err != nil {
		return err
	}
	defer vdb.Close()

	stats, err := BuildIndex(ctx, mgr.projectPath, mgr.cfg, vdb, mgr.invIndex)
	if err != nil {
		return err
	}

	// 保存索引状态
	if saveErr := mgr.stateStore.SaveFromStats(mgr.projectPath, stats, mgr.cfg.ScanExtensions); saveErr != nil {
		slog.Warn("保存索引状态失败", "err", saveErr)
	}

	mgr.mu.Lock()
	mgr.stale = false
	mgr.mu.Unlock()

	slog.Info("全量索引完成", "files", stats.TotalFiles, "chunks", stats.TotalChunks)
	return nil
}

// TriggerUpdateIfStale 搜索时检测过期，触发后台增量更新
func (mgr *IndexManager) TriggerUpdateIfStale(ctx context.Context) {
	if !mgr.IsStale() {
		return
	}

	// 使用 atomic.Bool 确保同一时间只有一个更新 goroutine
	if !mgr.updating.CompareAndSwap(false, true) {
		return // 已有更新在进行
	}

	go func() {
		defer mgr.updating.Store(false)

		slog.Info("检测到索引过期，开始后台增量更新")
		if err := mgr.incrementalUpdate(ctx); err != nil {
			slog.Error("增量更新失败", "err", err)
			// 保持 stale=true，下次搜索时重试
			return
		}

		mgr.mu.Lock()
		mgr.stale = false
		mgr.mu.Unlock()
		slog.Info("增量更新完成")
	}()
}

// IsStale 返回索引是否过期
func (mgr *IndexManager) IsStale() bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.stale
}

// GetInvertedIndex 获取倒排索引实例
func (mgr *IndexManager) GetInvertedIndex() *search.InvertedIndex {
	return mgr.invIndex
}

// incrementalUpdate 增量更新索引
func (mgr *IndexManager) incrementalUpdate(ctx context.Context) error {
	state, err := mgr.stateStore.Load()
	if err != nil {
		// 状态文件丢失，全量重建
		slog.Info("增量更新：状态文件丢失，转为全量构建")
		return mgr.fullBuild(ctx)
	}

	// 扫描当前文件 mtime
	currentMtimes, err := scanFileMtimes(mgr.projectPath, mgr.cfg.ScanExtensions)
	if err != nil {
		return err
	}

	// 找出变更文件
	var changedFiles []string
	for path, mtime := range currentMtimes {
		if oldMtime, ok := state.FileMtimes[path]; !ok || oldMtime != mtime {
			changedFiles = append(changedFiles, path)
		}
	}

	// 找出删除的文件
	for path := range state.FileMtimes {
		if _, ok := currentMtimes[path]; !ok {
			changedFiles = append(changedFiles, path)
		}
	}

	if len(changedFiles) == 0 {
		slog.Info("增量更新：无变更文件")
		return nil
	}

	slog.Info("增量更新", "changed", len(changedFiles))

	vdb, err := search.NewVectorDB(ctx, mgr.cfg)
	if err != nil {
		return err
	}
	defer vdb.Close()

	// 确保集合存在
	if err := vdb.EnsureCollection(ctx); err != nil {
		return err
	}

	// 删除变更文件的旧向量
	for _, filePath := range changedFiles {
		if err := vdb.DeleteByFile(ctx, filePath); err != nil {
			slog.Warn("删除旧向量失败", "file", filePath, "err", err)
		}
		mgr.invIndex.RemoveFile(filePath)
	}

	// 重新索引变更文件
	docs, err := ScanFiles(mgr.projectPath, mgr.cfg.ScanExtensions)
	if err != nil {
		return err
	}

	changedSet := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changedSet[f] = true
	}

	var allVectors [][]float32
	var allTexts []string
	var allIDs []string
	var allMetadatas []map[string]interface{}

	// 获取当前最大 doc ID
	chunkIdx := 0
	// 简单用时间戳作为 ID 前缀避免冲突
	idPrefix := time.Now().UnixMilli()

	for _, doc := range docs {
		if !changedSet[doc.FilePath] {
			continue
		}

		lang := structure.DetectLanguage(doc.FilePath)
		chunks := structure.SplitByStructure(doc.Content, lang, doc.FilePath, mgr.cfg.MaxChunkSize)

		// 构建倒排索引
		mgr.invIndex.BuildFromChunks(chunks, doc.FilePath)

		for _, chunk := range chunks {
			vector, err := embedding.GetEmbedding(mgr.cfg, chunk.Content)
			if err != nil {
				slog.Warn("向量化失败", "file", doc.FilePath, "err", err)
				continue
			}

			chunkIdx++
			allIDs = append(allIDs, fmt.Sprintf("doc_%d_%d", idPrefix, chunkIdx))
			allTexts = append(allTexts, chunk.Content)
			allVectors = append(allVectors, vector)
			allMetadatas = append(allMetadatas, chunk.Metadata)
		}
	}

	// 批量插入新向量
	if len(allVectors) > 0 {
		slog.Info("增量插入向量", "count", len(allVectors))
		if err := vdb.Insert(ctx, allIDs, allTexts, allVectors, allMetadatas); err != nil {
			return err
		}
	}

	// 更新索引状态（增量更新的 chunk 数不同于全量构建）
	stats := &IndexStats{
		TotalFiles:  len(currentMtimes),
		TotalChunks: state.TotalChunks - len(changedFiles) + chunkIdx,
	}
	if saveErr := mgr.stateStore.SaveFromStats(mgr.projectPath, stats, mgr.cfg.ScanExtensions); saveErr != nil {
		slog.Warn("保存索引状态失败", "err", saveErr)
	}

	return nil
}

// rebuildInvertedIndex 重建内存倒排索引（服务重启后向量索引仍在，但倒排索引丢失）
func (mgr *IndexManager) rebuildInvertedIndex(ctx context.Context) {
	slog.Info("重建倒排索引")
	docs, err := ScanFiles(mgr.projectPath, mgr.cfg.ScanExtensions)
	if err != nil {
		slog.Error("扫描文件失败", "err", err)
		return
	}

	for _, doc := range docs {
		lang := structure.DetectLanguage(doc.FilePath)
		chunks := structure.SplitByStructure(doc.Content, lang, doc.FilePath, mgr.cfg.MaxChunkSize)
		mgr.invIndex.BuildFromChunks(chunks, doc.FilePath)
	}

	slog.Info("倒排索引重建完成", "symbols", mgr.invIndex.Size())
}
