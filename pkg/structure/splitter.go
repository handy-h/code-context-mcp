package structure

import (
	"path/filepath"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// DetectLanguage 根据文件扩展名检测语言
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".vue":
		return "vue"
	case ".js":
		return "js"
	case ".ts":
		return "ts"
	case ".md":
		return "md"
	case ".py":
		return "py"
	case ".rs":
		return "rust"
	default:
		return ""
	}
}

// SplitByStructure 按语法结构切分代码
func SplitByStructure(content string, lang string, filePath string, maxChunkSize int) []types.CodeChunk {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	if lang == "" {
		lang = DetectLanguage(filePath)
	}

	var chunks []types.CodeChunk
	switch lang {
	case "go":
		chunks = chunkGo(content, filePath)
	case "vue":
		chunks = chunkVue(content, filePath)
	case "js", "ts":
		chunks = chunkJSTS(content, filePath, lang)
	case "md":
		chunks = chunkMarkdown(content, filePath)
	case "py":
		chunks = chunkPython(content, filePath)
	case "rust":
		chunks = chunkRust(content, filePath)
	default:
		// 降级为固定字符窗口切分
		return chunkByFixedSize(content, filePath, maxChunkSize)
	}

	if len(chunks) == 0 {
		// 正则匹配无结果，整个文件作为一个切分块
		lines := strings.Split(content, "\n")
		chunks = []types.CodeChunk{{
			Content: content,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     filepath.Base(filePath),
				"line_start": 1,
				"line_end":   len(lines),
				"type":       "file",
				"language":   lang,
			},
		}}
	}

	// 超长块二次切分
	return splitOversizedChunks(chunks, maxChunkSize)
}

// ================= 辅助函数 =================

// chunkByFixedSize 固定字符窗口切分（降级策略）
func chunkByFixedSize(content string, filePath string, chunkSize int) []types.CodeChunk {
	var chunks []types.CodeChunk
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// 为每行计算 rune 起始偏移量，用于准确定位 chunk 的行号范围
	lineRuneOffsets := make([]int, totalLines+1)
	for i, line := range lines {
		lineRuneOffsets[i+1] = lineRuneOffsets[i] + len([]rune(line)) + 1 // +1 for newline
	}

	runes := []rune(content)

	// 双指针法：lineIdx 跟踪当前行，避免每次 chunk 都遍历所有行
	lineIdx := 0
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		if len(strings.TrimSpace(chunk)) < 10 {
			continue
		}

		// 前移 lineIdx 到 chunk 起始行
		for lineIdx < totalLines && lineRuneOffsets[lineIdx+1] <= i {
			lineIdx++
		}
		chunkLineStart := lineIdx + 1

		// 找到 chunk 结束行
		chunkLineEnd := chunkLineStart
		for chunkLineEnd < totalLines && lineRuneOffsets[chunkLineEnd] < end {
			chunkLineEnd++
		}

		chunks = append(chunks, types.CodeChunk{
			Content: chunk,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     filepath.Base(filePath),
				"line_start": chunkLineStart,
				"line_end":   chunkLineEnd,
				"type":       "file",
				"language":   DetectLanguage(filePath),
			},
		})
	}
	return chunks
}

// splitOversizedChunks 对超长块进行二次切分（按行边界，避免从代码行中间截断）
func splitOversizedChunks(chunks []types.CodeChunk, maxChunkSize int) []types.CodeChunk {
	var result []types.CodeChunk
	for _, chunk := range chunks {
		if len([]rune(chunk.Content)) <= maxChunkSize {
			result = append(result, chunk)
			continue
		}

		lines := strings.Split(chunk.Content, "\n")
		lineStart, ok := chunk.Metadata["line_start"].(int)
		if !ok {
			lineStart = 1
		}

		var buf []string
		bufRunes := 0
		chunkIdx := 0
		subLineStart := lineStart

		for i, line := range lines {
			lineRunes := len([]rune(line)) + 1 // +1 for newline
			if bufRunes+lineRunes > maxChunkSize && len(buf) > 0 {
				subContent := strings.Join(buf, "\n")
				if strings.TrimSpace(subContent) != "" {
					meta := make(map[string]interface{}, len(chunk.Metadata)+2)
					for k, v := range chunk.Metadata {
						meta[k] = v
					}
					meta["line_start"] = subLineStart
					meta["line_end"] = subLineStart + len(buf) - 1
					meta["chunk_index"] = chunkIdx
					result = append(result, types.CodeChunk{Content: subContent, Metadata: meta})
					chunkIdx++
				}
				subLineStart = lineStart + i
				buf = buf[:0]
				bufRunes = 0
			}
			buf = append(buf, line)
			bufRunes += lineRunes
		}

		if len(buf) > 0 {
			subContent := strings.Join(buf, "\n")
			if strings.TrimSpace(subContent) != "" {
				meta := make(map[string]interface{}, len(chunk.Metadata)+2)
				for k, v := range chunk.Metadata {
					meta[k] = v
				}
				meta["line_start"] = subLineStart
				meta["line_end"] = subLineStart + len(buf) - 1
				meta["chunk_index"] = chunkIdx
				result = append(result, types.CodeChunk{Content: subContent, Metadata: meta})
			}
		}
	}
	return result
}
