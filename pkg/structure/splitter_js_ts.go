package structure

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= JS/TS 切分策略 =================
// 策略：匹配 export/function/class/箭头函数等声明边界

var (
	jsExportFuncRe  = regexp.MustCompile(`export\s+(default\s+)?function\s+(\w+)`)
	jsExportClassRe = regexp.MustCompile(`export\s+class\s+(\w+)`)
	jsExportVarRe   = regexp.MustCompile(`export\s+(const|let|var)\s+(\w+)`)
	jsFuncRe        = regexp.MustCompile(`(async\s+)?function\s+(\w+)`)
	jsClassRe       = regexp.MustCompile(`class\s+(\w+)`)
	jsConstRe       = regexp.MustCompile(`^(const|let|var)\s+(\w+)\s*=`)
)

func chunkJSTS(content string, filePath string, lang string) []types.CodeChunk {
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

	var chunks []types.CodeChunk

	// 前导内容
	if boundaries[0].lineIdx > 0 {
		headerContent := strings.Join(lines[:boundaries[0].lineIdx], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, types.CodeChunk{
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

		chunks = append(chunks, types.CodeChunk{
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
