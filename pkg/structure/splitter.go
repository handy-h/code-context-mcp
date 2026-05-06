package structure

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// CodeChunk is an alias for types.CodeChunk for backward compatibility
type CodeChunk = types.CodeChunk

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

	var chunks []CodeChunk
	switch lang {
	case "go":
		chunks = splitGo(content, filePath)
	case "vue":
		chunks = splitVue(content, filePath)
	case "js", "ts":
		chunks = splitJSTS(content, filePath, lang)
	case "md":
		chunks = splitMarkdown(content, filePath)
	case "py":
		chunks = splitPython(content, filePath)
	default:
		// 降级为固定字符窗口切分
		return splitByFixedSize(content, filePath, maxChunkSize)
	}

	if len(chunks) == 0 {
		// 正则匹配无结果，整个文件作为一个切分块
		lines := strings.Split(content, "\n")
		chunks = []CodeChunk{{
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

// ================= Go 切分策略 =================

var (
	goFuncRe      = regexp.MustCompile(`^func\s+(\(\w+\s+\*?\w+\)\s+)?(\w+)`)
	goStructRe    = regexp.MustCompile(`^type\s+(\w+)\s+struct`)
	goInterfaceRe = regexp.MustCompile(`^type\s+(\w+)\s+interface`)
	goVarRe       = regexp.MustCompile(`^(var|const)\s+`)
	// goPackageRe     = regexp.MustCompile(`^package\s+`)
	// goImportRe      = regexp.MustCompile(`^import\s+`)
)

func splitGo(content string, filePath string) []CodeChunk {
	lines := strings.Split(content, "\n")
	type boundary struct {
		lineIdx int
		symbol  string
		kind    string
	}

	var boundaries []boundary

	// 收集所有结构边界
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := goFuncRe.FindStringSubmatch(trimmed); m != nil {
			symbol := m[2] // 函数名
			if m[1] != "" {
				symbol = m[2] // 方法名
			}
			boundaries = append(boundaries, boundary{i, symbol, "function"})
		} else if m := goStructRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "struct"})
		} else if m := goInterfaceRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "interface"})
		} else if goVarRe.MatchString(trimmed) {
			// 仅顶层 var/const
			if i == 0 || !isContinuation(lines, i) {
				symbol := extractGoVarSymbol(trimmed)
				boundaries = append(boundaries, boundary{i, symbol, "variable"})
			}
		}
	}

	// 如果没有找到任何边界，检查是否有 package/import 头
	if len(boundaries) == 0 {
		return nil
	}

	// 检查第一个边界前是否有 package/import 头
	var chunks []CodeChunk
	firstBoundary := boundaries[0].lineIdx
	if firstBoundary > 0 {
		headerContent := strings.Join(lines[:firstBoundary], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, CodeChunk{
				Content: headerContent,
				Metadata: map[string]interface{}{
					"file":       filePath,
					"symbol":     "header",
					"line_start": 1,
					"line_end":   firstBoundary,
					"type":       "header",
					"language":   "go",
				},
			})
		}
	}

	// 按边界切分
	for i, b := range boundaries {
		startLine := b.lineIdx
		var endLine int
		if i+1 < len(boundaries) {
			endLine = boundaries[i+1].lineIdx
		} else {
			endLine = len(lines)
		}

		chunkContent := strings.Join(lines[startLine:endLine], "\n")
		if strings.TrimSpace(chunkContent) == "" {
			continue
		}

		chunks = append(chunks, CodeChunk{
			Content: chunkContent,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     b.symbol,
				"line_start": startLine + 1,
				"line_end":   endLine,
				"type":       b.kind,
				"language":   "go",
			},
		})
	}

	return chunks
}

func isContinuation(lines []string, idx int) bool {
	// 检查上一行是否是 var/const 块的续行（在括号内或上一行以逗号结尾）
	if idx == 0 {
		return false
	}
	prev := strings.TrimSpace(lines[idx-1])
	return strings.HasSuffix(prev, ",") || strings.HasSuffix(prev, "(") || prev == ""
}

func extractGoVarSymbol(line string) string {
	// var/const Name = ... 或 var/const ( ...
	re := regexp.MustCompile(`^(var|const)\s+(\w+)`)
	if m := re.FindStringSubmatch(line); m != nil {
		return m[2]
	}
	return "variables"
}

// ================= Vue 切分策略 =================

var (
	vueTemplateRe = regexp.MustCompile(`(?i)<template[^>]*>`)
	vueScriptRe   = regexp.MustCompile(`(?i)<script[^>]*>`)
	vueStyleRe    = regexp.MustCompile(`(?i)<style[^>]*>`)
	vueCloseRe    = regexp.MustCompile(`(?i)</(template|script|style)\s*>`)
)

func splitVue(content string, filePath string) []CodeChunk {
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

	var chunks []CodeChunk

	// 前导内容（template 之前）
	if blocks[0].startIdx > 0 {
		headerContent := strings.Join(lines[:blocks[0].startIdx], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, CodeChunk{
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
				subChunks := splitJSTS(scriptContent, filePath, "js")
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

		chunks = append(chunks, CodeChunk{
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

func ExtractScriptContent(scriptBlock string) string {
	// 提取 <script> 标签内的内容
	startRe := regexp.MustCompile(`(?i)<script[^>]*>`)
	endRe := regexp.MustCompile(`(?i)</script\s*>`)
	startMatch := startRe.FindStringIndex(scriptBlock)
	endMatch := endRe.FindStringIndex(scriptBlock)
	if startMatch != nil && endMatch != nil {
		return scriptBlock[startMatch[1]:endMatch[0]]
	}
	return ""
}

// ================= JS/TS 切分策略 =================

var (
	jsExportFuncRe  = regexp.MustCompile(`export\s+(default\s+)?function\s+(\w+)`)
	jsExportClassRe = regexp.MustCompile(`export\s+class\s+(\w+)`)
	jsExportVarRe   = regexp.MustCompile(`export\s+(const|let|var)\s+(\w+)`)
	jsFuncRe        = regexp.MustCompile(`(async\s+)?function\s+(\w+)`)
	jsClassRe       = regexp.MustCompile(`class\s+(\w+)`)
	jsConstRe       = regexp.MustCompile(`^(const|let|var)\s+(\w+)\s*=`)
)

func splitJSTS(content string, filePath string, lang string) []CodeChunk {
	lines := strings.Split(content, "\n")

	type boundary struct {
		lineIdx int
		symbol  string
		kind    string
	}

	var boundaries []boundary

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := jsExportFuncRe.FindStringSubmatch(trimmed); m != nil {
			symbol := m[2]
			if m[1] != "" {
				symbol = "default"
			}
			boundaries = append(boundaries, boundary{i, symbol, "function"})
		} else if m := jsExportClassRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "class"})
		} else if m := jsExportVarRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[2], "export"})
		} else if m := jsFuncRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[2], "function"})
		} else if m := jsClassRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "class"})
		} else if m := jsConstRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[2], "variable"})
		}
	}

	if len(boundaries) == 0 {
		return nil
	}

	var chunks []CodeChunk

	// 前导内容
	if boundaries[0].lineIdx > 0 {
		headerContent := strings.Join(lines[:boundaries[0].lineIdx], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, CodeChunk{
				Content: headerContent,
				Metadata: map[string]interface{}{
					"file":       filePath,
					"symbol":     "header",
					"line_start": 1,
					"line_end":   boundaries[0].lineIdx,
					"type":       "header",
					"language":   lang,
				},
			})
		}
	}

	for i, b := range boundaries {
		startLine := b.lineIdx
		var endLine int
		if i+1 < len(boundaries) {
			endLine = boundaries[i+1].lineIdx
		} else {
			endLine = len(lines)
		}

		chunkContent := strings.Join(lines[startLine:endLine], "\n")
		if strings.TrimSpace(chunkContent) == "" {
			continue
		}

		chunks = append(chunks, CodeChunk{
			Content: chunkContent,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     b.symbol,
				"line_start": startLine + 1,
				"line_end":   endLine,
				"type":       b.kind,
				"language":   lang,
			},
		})
	}

	return chunks
}

// ================= Markdown 切分策略 =================

var mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)`)

func splitMarkdown(content string, filePath string) []CodeChunk {
	lines := strings.Split(content, "\n")

	type boundary struct {
		lineIdx int
		symbol  string
		level   int
	}

	var boundaries []boundary

	for i, line := range lines {
		if m := mdHeadingRe.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			boundaries = append(boundaries, boundary{i, strings.TrimSpace(m[2]), level})
		}
	}

	if len(boundaries) == 0 {
		return nil
	}

	var chunks []CodeChunk

	// 标题前的内容
	if boundaries[0].lineIdx > 0 {
		headerContent := strings.Join(lines[:boundaries[0].lineIdx], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, CodeChunk{
				Content: headerContent,
				Metadata: map[string]interface{}{
					"file":       filePath,
					"symbol":     "preamble",
					"line_start": 1,
					"line_end":   boundaries[0].lineIdx,
					"type":       "section",
					"language":   "md",
				},
			})
		}
	}

	for i, b := range boundaries {
		startLine := b.lineIdx
		var endLine int
		if i+1 < len(boundaries) {
			endLine = boundaries[i+1].lineIdx
		} else {
			endLine = len(lines)
		}

		chunkContent := strings.Join(lines[startLine:endLine], "\n")
		if strings.TrimSpace(chunkContent) == "" {
			continue
		}

		chunks = append(chunks, CodeChunk{
			Content: chunkContent,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     b.symbol,
				"line_start": startLine + 1,
				"line_end":   endLine,
				"type":       "section",
				"language":   "md",
			},
		})
	}

	return chunks
}

// ================= Python 切分策略 =================

var (
	pyFuncRe  = regexp.MustCompile(`^(async\s+)?def\s+(\w+)`)
	pyClassRe = regexp.MustCompile(`^class\s+(\w+)`)
)

func splitPython(content string, filePath string) []CodeChunk {
	lines := strings.Split(content, "\n")

	type boundary struct {
		lineIdx int
		symbol  string
		kind    string
	}

	var boundaries []boundary

	for i, line := range lines {
		if m := pyFuncRe.FindStringSubmatch(line); m != nil {
			boundaries = append(boundaries, boundary{i, m[2], "function"})
		} else if m := pyClassRe.FindStringSubmatch(line); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "class"})
		}
	}

	if len(boundaries) == 0 {
		return nil
	}

	var chunks []CodeChunk

	if boundaries[0].lineIdx > 0 {
		headerContent := strings.Join(lines[:boundaries[0].lineIdx], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, CodeChunk{
				Content: headerContent,
				Metadata: map[string]interface{}{
					"file":       filePath,
					"symbol":     "header",
					"line_start": 1,
					"line_end":   boundaries[0].lineIdx,
					"type":       "header",
					"language":   "py",
				},
			})
		}
	}

	for i, b := range boundaries {
		startLine := b.lineIdx
		var endLine int
		if i+1 < len(boundaries) {
			endLine = boundaries[i+1].lineIdx
		} else {
			endLine = len(lines)
		}

		chunkContent := strings.Join(lines[startLine:endLine], "\n")
		if strings.TrimSpace(chunkContent) == "" {
			continue
		}

		chunks = append(chunks, CodeChunk{
			Content: chunkContent,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     b.symbol,
				"line_start": startLine + 1,
				"line_end":   endLine,
				"type":       b.kind,
				"language":   "py",
			},
		})
	}

	return chunks
}

// ================= 辅助函数 =================

// splitByFixedSize 固定字符窗口切分（降级策略）
func splitByFixedSize(content string, filePath string, chunkSize int) []CodeChunk {
	var chunks []CodeChunk
	runes := []rune(content)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		if len(strings.TrimSpace(chunk)) < 10 {
			continue
		}
		chunks = append(chunks, CodeChunk{
			Content: chunk,
			Metadata: map[string]interface{}{
				"file":       filePath,
				"symbol":     filepath.Base(filePath),
				"line_start": 1,
				"line_end":   totalLines,
				"type":       "file",
				"language":   DetectLanguage(filePath),
			},
		})
	}
	return chunks
}

// splitOversizedChunks 对超长块进行二次切分
func splitOversizedChunks(chunks []CodeChunk, maxChunkSize int) []CodeChunk {
	var result []CodeChunk
	for _, chunk := range chunks {
		if len([]rune(chunk.Content)) <= maxChunkSize {
			result = append(result, chunk)
			continue
		}

		// 二次切分
		runes := []rune(chunk.Content)
		chunkIdx := 0
		for i := 0; i < len(runes); i += maxChunkSize {
			end := i + maxChunkSize
			if end > len(runes) {
				end = len(runes)
			}
			subContent := string(runes[i:end])
			if len(strings.TrimSpace(subContent)) < 10 {
				continue
			}

			// 复制元数据并追加 chunk_index
			meta := make(map[string]interface{}, len(chunk.Metadata)+1)
			for k, v := range chunk.Metadata {
				meta[k] = v
			}
			meta["chunk_index"] = chunkIdx

			result = append(result, CodeChunk{
				Content:  subContent,
				Metadata: meta,
			})
			chunkIdx++
		}
	}
	return result
}
