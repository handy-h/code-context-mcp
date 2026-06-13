package structure

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= Python 切分策略 =================
// 策略：按 def/class 声明边界切分

var (
	pyFuncRe  = regexp.MustCompile(`^(\s*)(async\s+)?def\s+(\w+)`)
	pyClassRe = regexp.MustCompile(`^(\s*)(class\s+(\w+))`)
)

func chunkPython(content string, filePath string) []types.CodeChunk {
	lines := strings.Split(content, "\n")

	type boundary struct {
		lineIdx int
		symbol  string
		kind    string
	}

	var boundaries []boundary

	for i, line := range lines {
		if m := pyFuncRe.FindStringSubmatch(line); m != nil {
			indent := m[1]
			// 匹配顶层（无缩进）和一级缩进（4空格或1个tab）的定义
			indentLen := 0
			for _, ch := range indent {
				if ch == ' ' {
					indentLen++
				} else if ch == '\t' {
					indentLen += 4
				}
			}
			if indentLen <= 4 {
				boundaries = append(boundaries, boundary{i, m[3], "function"})
			}
		} else if m := pyClassRe.FindStringSubmatch(line); m != nil {
			indent := m[1]
			indentLen := 0
			for _, ch := range indent {
				if ch == ' ' {
					indentLen++
				} else if ch == '\t' {
					indentLen += 4
				}
			}
			if indentLen <= 4 {
				boundaries = append(boundaries, boundary{i, m[3], "class"})
			}
		}
	}

	if len(boundaries) == 0 {
		return nil
	}

	var chunks []types.CodeChunk

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

		chunks = append(chunks, types.CodeChunk{
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
