package structure

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= Vue 切分策略 =================
// 策略：匹配 <template>, <script>, <style> 块边界
// script 块内部按 JS/TS 策略二次切分

var (
	vueTemplateRe = regexp.MustCompile(`(?i)<template[^>]*>`)
	vueScriptRe   = regexp.MustCompile(`(?i)<script[^>]*>`)
	vueStyleRe    = regexp.MustCompile(`(?i)<style[^>]*>`)
	vueCloseRe    = regexp.MustCompile(`(?i)</(template|script|style)\s*>`)
)

func chunkVue(content string, filePath string) []types.CodeChunk {
	lines := strings.Split(content, "\n")
	componentName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	type block struct {
		startIdx int
		endIdx   int
		kind     string
	}

	var blocks []block
	var currentBlock *block

	for i, line := range lines {
		if currentBlock == nil {
			if vueTemplateRe.MatchString(line) {
				currentBlock = &block{i, -1, "template"}
			} else if vueScriptRe.MatchString(line) {
				currentBlock = &block{i, -1, "script"}
			} else if vueStyleRe.MatchString(line) {
				currentBlock = &block{i, -1, "style"}
			}
		} else {
			if m := vueCloseRe.FindStringSubmatch(line); m != nil {
				tag := strings.ToLower(m[1])
				if (currentBlock.kind == "template" && tag == "template") ||
					(currentBlock.kind == "script" && tag == "script") ||
					(currentBlock.kind == "style" && tag == "style") {
					currentBlock.endIdx = i + 1
					blocks = append(blocks, *currentBlock)
					currentBlock = nil
				}
			}
		}
	}

	// 未闭合的块
	if currentBlock != nil {
		currentBlock.endIdx = len(lines)
		blocks = append(blocks, *currentBlock)
	}

	if len(blocks) == 0 {
		return nil
	}

	var chunks []types.CodeChunk

	// 前导内容（template 之前）
	if blocks[0].startIdx > 0 {
		headerContent := strings.Join(lines[:blocks[0].startIdx], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, types.CodeChunk{
				Content: headerContent,
				Metadata: map[string]interface{}{
					"file":       filePath,
					"symbol":     "header",
					"line_start": 1,
					"line_end":   blocks[0].startIdx,
					"type":       "header",
					"language":   "vue",
				},
			})
		}
	}

	for _, b := range blocks {
		blockContent := strings.Join(lines[b.startIdx:b.endIdx], "\n")
		if strings.TrimSpace(blockContent) == "" {
			continue
		}

		if b.kind == "script" {
			// script 块内部按 JS/TS 函数/组件切分
			scriptContent := ExtractScriptContent(blockContent)
			if scriptContent != "" {
				subChunks := chunkJSTS(scriptContent, filePath, "js")
				for _, sc := range subChunks {
					// 调整行号偏移
					offset := b.startIdx
					if ls, ok := sc.Metadata["line_start"].(int); ok {
						sc.Metadata["line_start"] = ls + offset
					}
					if le, ok := sc.Metadata["line_end"].(int); ok {
						sc.Metadata["line_end"] = le + offset
					}
					sc.Metadata["language"] = "vue"
					chunks = append(chunks, sc)
				}
				continue
			}
		}

		chunks = append(chunks, types.CodeChunk{
			Content: blockContent,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     componentName,
				"line_start": b.startIdx + 1,
				"line_end":   b.endIdx,
				"type":       b.kind,
				"language":   "vue",
			},
		})
	}

	return chunks
}

// ExtractScriptContent 提取 <script> 标签内的内容
func ExtractScriptContent(scriptBlock string) string {
	startMatch := vueScriptRe.FindStringIndex(scriptBlock)
	if startMatch == nil {
		return ""
	}
	// 查找 startMatch 之后的第一个闭合标签
	allClose := vueCloseRe.FindAllStringIndex(scriptBlock, -1)
	for _, m := range allClose {
		if m[0] >= startMatch[1] {
			return scriptBlock[startMatch[1]:m[0]]
		}
	}
	return ""
}
