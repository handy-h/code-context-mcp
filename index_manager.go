package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// IndexManager 索引管理器
type IndexManager struct {
	cfg         Config
	stateStore  *IndexStateStore
	invIndex    *InvertedIndex
	mu          sync.RWMutex
	stale       bool
	updating    atomic.Bool
	projectPath string
}

// NewIndexManager 创建索引管理器
func NewIndexManager(cfg Config, projectPath string) *IndexManager {
	stateStore := NewIndexStateStore(projectPath, cfg.IndexStatePath)
	return &IndexManager{
		cfg:         cfg,
		stateStore:  stateStore,
		invIndex:    NewInvertedIndex(),
		projectPath: projectPath,
		stale:       true,
	}
}

// CheckAndAutoIndex 启动时检查索引状态并自动索引
func (im *IndexManager) CheckAndAutoIndex(ctx context.Context) error {
	if !im.cfg.AutoIndex {
		log.Println("自动索引已禁用")
		return nil
	}

	// 尝试加载索引状态
	state, err := im.stateStore.Load()
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			return err
		}
		// 状态文件不存在或损坏，触发全量构建
		log.Println("索引状态不存在或已损坏，开始全量索引构建...")
		return im.fullBuild(ctx)
	}

	// 比对指纹
	currentFingerprint, _, fpErr := im.stateStore.GetCurrentFingerprint(im.projectPath, im.cfg.ScanExtensions)
	if fpErr != nil {
		log.Printf("计算索引指纹失败: %v，标记索引为过期", fpErr)
		im.mu.Lock()
		im.stale = true
		im.mu.Unlock()
		return nil
	}

	if currentFingerprint == state.Fingerprint {
		log.Println("索引状态有效，无需重建")
		im.mu.Lock()
		im.stale = false
		im.mu.Unlock()
		// 重建倒排索引（内存中，重启后丢失）
		go im.rebuildInvertedIndex(ctx)
		return nil
	}

	log.Println("索引已过期（指纹不匹配），开始全量索引构建...")
	return im.fullBuild(ctx)
}

// fullBuild 全量构建索引
func (im *IndexManager) fullBuild(ctx context.Context) error {
	vdb, err := NewVectorDB(ctx, im.cfg)
	if err != nil {
		return err
	}
	defer vdb.Close()

	stats, err := BuildIndex(ctx, im.projectPath, im.cfg, vdb, im.invIndex)
	if err != nil {
		return err
	}

	// 保存索引状态
	currentFingerprint, currentMtimes, _ := im.stateStore.GetCurrentFingerprint(im.projectPath, im.cfg.ScanExtensions)
	state := &IndexState{
		LastIndexedAt: time.Now(),
		Fingerprint:   currentFingerprint,
		TotalFiles:    stats.TotalFiles,
		TotalChunks:   stats.TotalChunks,
		ProjectPath:   im.projectPath,
		FileMtimes:    currentMtimes,
	}
	if saveErr := im.stateStore.Save(state); saveErr != nil {
		log.Printf("保存索引状态失败: %v", saveErr)
	}

	im.mu.Lock()
	im.stale = false
	im.mu.Unlock()

	log.Printf("全量索引完成: %d 文件, %d 片段", stats.TotalFiles, stats.TotalChunks)
	return nil
}

// TriggerUpdateIfStale 搜索时检测过期，触发后台增量更新
func (im *IndexManager) TriggerUpdateIfStale(ctx context.Context) {
	if !im.IsStale() {
		return
	}

	// 使用 atomic.Bool 确保同一时间只有一个更新 goroutine
	if !im.updating.CompareAndSwap(false, true) {
		return // 已有更新在进行
	}

	go func() {
		defer im.updating.Store(false)

		log.Println("检测到索引过期，开始后台增量更新...")
		if err := im.incrementalUpdate(ctx); err != nil {
			log.Printf("增量更新失败: %v", err)
			// 保持 stale=true，下次搜索时重试
			return
		}

		im.mu.Lock()
		im.stale = false
		im.mu.Unlock()
		log.Println("增量更新完成")
	}()
}

// IsStale 返回索引是否过期
func (im *IndexManager) IsStale() bool {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.stale
}

// GetInvertedIndex 获取倒排索引实例
func (im *IndexManager) GetInvertedIndex() *InvertedIndex {
	return im.invIndex
}

// incrementalUpdate 增量更新索引
func (im *IndexManager) incrementalUpdate(ctx context.Context) error {
	state, err := im.stateStore.Load()
	if err != nil {
		// 状态文件丢失，全量重建
		log.Println("增量更新: 状态文件丢失，转为全量构建")
		return im.fullBuild(ctx)
	}

	// 扫描当前文件 mtime
	currentMtimes, err := scanFileMtimes(im.projectPath, im.cfg.ScanExtensions)
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
		log.Println("增量更新: 无变更文件")
		return nil
	}

	log.Printf("增量更新: %d 个文件变更", len(changedFiles))

	vdb, err := NewVectorDB(ctx, im.cfg)
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
			log.Printf("删除文件 %s 的旧向量失败: %v", filePath, err)
		}
		im.invIndex.RemoveFile(filePath)
	}

	// 重新索引变更文件
	docs, err := ScanFiles(im.projectPath, im.cfg.ScanExtensions)
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

		lang := detectLanguage(doc.FilePath)
		chunks := SplitByStructure(doc.Content, lang, doc.FilePath, im.cfg.MaxChunkSize)

		// 构建倒排索引
		im.invIndex.BuildFromChunks(chunks, doc.FilePath)

		for _, chunk := range chunks {
			vector, err := GetEmbedding(im.cfg, chunk.Content)
			if err != nil {
				log.Printf("向量化失败 [%s]: %v", doc.FilePath, err)
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
		log.Printf("增量插入 %d 条向量...", len(allVectors))
		if err := vdb.Insert(ctx, allIDs, allTexts, allVectors, allMetadatas); err != nil {
			return err
		}
	}

	// 更新索引状态
	currentFingerprint, _, _ := im.stateStore.GetCurrentFingerprint(im.projectPath, im.cfg.ScanExtensions)
	newState := &IndexState{
		LastIndexedAt: time.Now(),
		Fingerprint:   currentFingerprint,
		TotalFiles:    state.TotalFiles,
		TotalChunks:   state.TotalChunks - len(changedFiles) + chunkIdx,
		ProjectPath:   im.projectPath,
		FileMtimes:    currentMtimes,
	}
	if saveErr := im.stateStore.Save(newState); saveErr != nil {
		log.Printf("保存索引状态失败: %v", saveErr)
	}

	return nil
}

// rebuildInvertedIndex 重建内存倒排索引（服务重启后向量索引仍在，但倒排索引丢失）
func (im *IndexManager) rebuildInvertedIndex(ctx context.Context) {
	log.Println("重建内存倒排索引...")
	docs, err := ScanFiles(im.projectPath, im.cfg.ScanExtensions)
	if err != nil {
		log.Printf("重建倒排索引失败: %v", err)
		return
	}

	for _, doc := range docs {
		lang := detectLanguage(doc.FilePath)
		chunks := SplitByStructure(doc.Content, lang, doc.FilePath, im.cfg.MaxChunkSize)
		im.invIndex.BuildFromChunks(chunks, doc.FilePath)
	}

	log.Printf("倒排索引重建完成: %d 个符号", im.invIndex.Size())
}
