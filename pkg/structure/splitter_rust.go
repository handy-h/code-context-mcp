package structure

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= Rust 切分策略 =================
// 策略：按 fn/struct/enum/trait/impl/mod/type/const/static/macro_rules 边界切分
// use 语句归入 header 块

var (
	rustFuncRe    = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?(async\s+)?fn\s+(\w+)`)
	rustStructRe  = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?struct\s+(\w+)`)
	rustEnumRe    = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?enum\s+(\w+)`)
	rustTraitRe   = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?trait\s+(\w+)`)
	rustImplRe    = regexp.MustCompile(`^impl(\s+[\w<>,\s]+)?(\s+for\s+[\w<>,\s]+)?`)
	rustModRe     = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?mod\s+(\w+)`)
	rustTypeRe    = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?type\s+(\w+)`)
	rustConstRe   = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?(const|static)\s+(\w+)`)
	rustMacroRe   = regexp.MustCompile(`^macro_rules!\s+(\w+)`)
	rustUseRe     = regexp.MustCompile(`^use\s+`)
	rustImplNameRe = regexp.MustCompile(`^impl\s+(?:[\w<>,\s]+\s+for\s+)?(\w+)`)
)

func chunkRust(content string, filePath string) []types.CodeChunk {
	lines := strings.Split(content, "\n")
	type boundary struct {
		lineIdx int
		symbol  string
		kind    string
	}

	var boundaries []boundary
	useEndLine := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		if rustUseRe.MatchString(trimmed) {
			useEndLine = i + 1
			continue
		}

		if m := rustFuncRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[4], "function"})
			continue
		}

		if m := rustStructRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[3], "struct"})
			continue
		}

		if m := rustEnumRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[3], "enum"})
			continue
		}

		if m := rustTraitRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[3], "trait"})
			continue
		}

		if rustImplRe.MatchString(trimmed) {
			symbol := extractRustImplSymbol(trimmed)
			boundaries = append(boundaries, boundary{i, symbol, "impl"})
			continue
		}

		if m := rustModRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[3], "module"})
			continue
		}

		if m := rustTypeRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[3], "type"})
			continue
		}

		if m := rustConstRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[4], "constant"})
			continue
		}

		if m := rustMacroRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "macro"})
		}
	}

	if len(boundaries) == 0 && useEndLine == 0 {
		return nil
	}

	var chunks []types.CodeChunk
	firstBoundary := len(lines)
	if len(boundaries) > 0 {
		firstBoundary = boundaries[0].lineIdx
	}

	headerEnd := useEndLine
	if firstBoundary > headerEnd {
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
					"language":   "rust",
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
				"language":   "rust",
			},
		})
	}

	return chunks
}

func extractRustImplSymbol(line string) string {
	if m := rustImplNameRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return "impl"
}
