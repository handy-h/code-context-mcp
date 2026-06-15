package search

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/handy-h/code-context-mcp/internal/types"
)

var identifierRe = regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)

var keywords = map[string]struct{}{
	// Go 保留字
	"func": {}, "type": {}, "struct": {}, "interface": {},
	"package": {}, "range": {}, "go": {}, "defer": {},
	"chan": {}, "map": {}, "nil": {},
	"bool": {}, "byte": {}, "float64": {},
	"int": {}, "int8": {}, "int16": {}, "int32": {}, "int64": {},
	"uint": {}, "uint8": {}, "uint16": {}, "uint32": {}, "uint64": {},
	"uintptr": {}, "rune": {}, "complex64": {}, "complex128": {},
	"make": {}, "new": {}, "len": {}, "cap": {}, "append": {},
	// JS/TS 保留字
	"function": {}, "class": {}, "export": {},
	"async": {}, "await": {}, "null": {}, "undefined": {},
	"this": {}, "throw": {}, "try": {}, "catch": {},
	"finally": {}, "typeof": {}, "instanceof": {},
	"void": {}, "delete": {}, "yield": {},
	"while": {},
	// Python 保留字
	"def": {}, "self": {}, "None": {},
	"raise": {}, "with": {}, "as": {}, "lambda": {},
	"elif": {}, "pass": {},
	// 多语言共享保留字
	"var": {}, "const": {}, "import": {}, "return": {},
	"if": {}, "else": {}, "for": {}, "switch": {},
	"case": {}, "default": {}, "break": {}, "continue": {},
	"true": {}, "false": {},
	// Common
	"get": {}, "set": {}, "not": {}, "and": {}, "or": {},
	"do": {}, "end": {}, "then": {}, "when": {}, "is": {},
	"has": {}, "can": {}, "use": {},
}

// InvertedIndex 符号倒排索引。
//
// 构建规则（见 BuildFromChunks）：
//   - 每个 CodeChunk 的 Metadata["symbol"] 标识该块的"所属符号"（函数名、类型名等）
//   - 块内出现的标识符若等于 chunkSymbol → 标记为 definition
//   - 块内出现的其他标识符 → 标记为 reference
//
// 搜索支持驼峰/下划线自动转换：DiagnosisNotes ↔ diagnosis_notes
type InvertedIndex struct {
	mu        sync.RWMutex
	index     map[string][]types.SymbolOccurrence // 符号名 → 出现位置列表
	fileIndex map[string][]string                 // 文件路径 → 该文件的符号列表
}

// NewInvertedIndex 创建空倒排索引
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		index:     make(map[string][]types.SymbolOccurrence),
		fileIndex: make(map[string][]string),
	}
}

// BuildFromChunks 从切分块构建索引
func (ii *InvertedIndex) BuildFromChunks(chunks []types.CodeChunk, filePath string) {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	// 先移除该文件的旧索引
	ii.removeFileLocked(filePath)

	fileSymbols := make(map[string]bool)

	for _, chunk := range chunks {
		lines := strings.Split(chunk.Content, "\n")
		chunkSymbol := ""
		if s, ok := chunk.Metadata["symbol"].(string); ok {
			chunkSymbol = s
		}

		for lineIdx, line := range lines {
			identifiers := identifierRe.FindAllString(line, -1)
			lineNum := lineIdx + 1
			if ls, ok := chunk.Metadata["line_start"].(int); ok {
				lineNum = ls + lineIdx
			}

			for _, id := range identifiers {
				// 跳过过短的标识符和常见关键字
				if len(id) < 2 || isKeyword(id) {
					continue
				}

				occType := "reference"
				if id == chunkSymbol {
					occType = "definition"
				}

				context := extractContext(lines, lineIdx, 2)

				occ := types.SymbolOccurrence{
					Symbol:  id,
					File:    filePath,
					Line:    lineNum,
					Type:    occType,
					Context: context,
				}

				ii.index[id] = append(ii.index[id], occ)
				fileSymbols[id] = true
			}
		}
	}

	// 更新文件索引
	symbols := make([]string, 0, len(fileSymbols))
	for s := range fileSymbols {
		symbols = append(symbols, s)
	}
	ii.fileIndex[filePath] = symbols
}

// Search 搜索符号
func (ii *InvertedIndex) Search(query string, searchType string, topK int) []types.SymbolOccurrence {
	ii.mu.RLock()
	defer ii.mu.RUnlock()

	if topK <= 0 {
		topK = 20
	}

	// 扩展查询（驼峰/下划线转换）
	queries := expandQuery(query)

	var results []types.SymbolOccurrence
	seen := make(map[string]bool) // 去重

	for _, q := range queries {
		occurrences, ok := ii.index[q]
		if !ok {
			continue
		}
		for _, occ := range occurrences {
			key := fmt.Sprintf("%s:%d:%s", occ.File, occ.Line, occ.Symbol)
			if seen[key] {
				continue
			}
			seen[key] = true

			// 按 searchType 过滤
			if searchType == "definition" && occ.Type != "definition" {
				continue
			}
			if searchType == "reference" && occ.Type != "reference" {
				continue
			}

			results = append(results, occ)
		}
	}

	// 按文件排序
	sort.Slice(results, func(i, j int) bool {
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		return results[i].Line < results[j].Line
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// GetAllOccurrences 获取符号所有出现位置
func (ii *InvertedIndex) GetAllOccurrences(symbol string) []types.SymbolOccurrence {
	ii.mu.RLock()
	defer ii.mu.RUnlock()

	queries := expandQuery(symbol)

	var results []types.SymbolOccurrence
	seen := make(map[string]bool)

	for _, q := range queries {
		occurrences, ok := ii.index[q]
		if !ok {
			continue
		}
		for _, occ := range occurrences {
			key := fmt.Sprintf("%s:%d:%s", occ.File, occ.Line, occ.Symbol)
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, occ)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		return results[i].Line < results[j].Line
	})

	return results
}

// RemoveFile 移除文件相关的索引项
func (ii *InvertedIndex) RemoveFile(filePath string) {
	ii.mu.Lock()
	defer ii.mu.Unlock()
	ii.removeFileLocked(filePath)
}

func (ii *InvertedIndex) removeFileLocked(filePath string) {
	symbols, ok := ii.fileIndex[filePath]
	if !ok {
		return
	}

	for _, sym := range symbols {
		occurrences := ii.index[sym]
		var filtered []types.SymbolOccurrence
		for _, occ := range occurrences {
			if occ.File != filePath {
				filtered = append(filtered, occ)
			}
		}
		if len(filtered) == 0 {
			delete(ii.index, sym)
		} else {
			ii.index[sym] = filtered
		}
	}

	delete(ii.fileIndex, filePath)
}

// Size 返回索引中的符号数量
func (ii *InvertedIndex) Size() int {
	ii.mu.RLock()
	defer ii.mu.RUnlock()
	return len(ii.index)
}

// expandQuery 驼峰/下划线风格转换扩展查询
func expandQuery(query string) []string {
	result := []string{query}

	// 驼峰转下划线: DiagnosisNotes → diagnosis_notes
	snake := camelToSnake(query)
	if snake != query {
		result = append(result, snake)
	}

	// 下划线转驼峰: diagnosis_notes → DiagnosisNotes
	camel := snakeToCamel(query)
	if camel != query {
		result = append(result, camel)
	}

	return result
}

func camelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// extractContext 提取上下文摘要（目标行及前后各 n 行）
func extractContext(lines []string, targetIdx int, n int) string {
	start := targetIdx - n
	if start < 0 {
		start = 0
	}
	end := targetIdx + n + 1
	if end > len(lines) {
		end = len(lines)
	}

	context := strings.Join(lines[start:end], "\n")
	// 截断过长的上下文（按 rune 避免切断 UTF-8 多字节字符）
	if runeCount := len([]rune(context)); runeCount > 200 {
		context = string([]rune(context)[:200]) + "..."
	}
	return context
}

// isKeyword 检查是否为常见关键字（应跳过不索引）
func isKeyword(id string) bool {
	_, ok := keywords[id]
	return ok
}
