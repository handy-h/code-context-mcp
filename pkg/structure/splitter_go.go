package structure

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= Go 切分策略 =================
// 策略：按函数、结构体、接口、顶层 var/const 声明边界切分
// isContinuation 用于防止括号块内的 var/const 被误识别为新的切分点

var (
	goFuncRe      = regexp.MustCompile(`^func\s+(\(\w+\s+\*?\w+\)\s+)?(\w+)`)
	goStructRe    = regexp.MustCompile(`^type\s+(\w+)\s+struct`)
	goInterfaceRe = regexp.MustCompile(`^type\s+(\w+)\s+interface`)
	goVarRe       = regexp.MustCompile(`^(var|const)\s+`)
)

func chunkGo(content string, filePath string) []types.CodeChunk {
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
	var chunks []types.CodeChunk
	firstBoundary := boundaries[0].lineIdx
	if firstBoundary > 0 {
		headerContent := strings.Join(lines[:firstBoundary], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, types.CodeChunk{
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

		chunks = append(chunks, types.CodeChunk{
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
