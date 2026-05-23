package structure

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= Markdown 切分策略 =================
// 策略：按 ## / ### 等标题边界切分，标题前的内容作为 preamble

var mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)`)

func chunkMarkdown(content string, filePath string) []types.CodeChunk {
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

	var chunks []types.CodeChunk

	// 标题前的内容
	if boundaries[0].lineIdx > 0 {
		headerContent := strings.Join(lines[:boundaries[0].lineIdx], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, types.CodeChunk{
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

		chunks = append(chunks, types.CodeChunk{
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
