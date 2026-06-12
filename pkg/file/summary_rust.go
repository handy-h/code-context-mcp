package file

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= Rust 摘要提取 =================

var (
	rustUseSummaryRe    = regexp.MustCompile(`^use\s+([\w:]+(?:\{[^}]+\})?)`)
	rustFuncSummaryRe   = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?(async\s+)?fn\s+(\w+)`)
	rustStructSummaryRe = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?struct\s+(\w+)`)
	rustEnumSummaryRe   = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?enum\s+(\w+)`)
	rustTraitSummaryRe  = regexp.MustCompile(`^(pub(\s*\([^)]*\))?\s+)?trait\s+(\w+)`)
)

func summarizeRust(content string, summary *types.FileSummary) {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if m := rustUseSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Imports = append(summary.Imports, m[1])
		}

		if m := rustFuncSummaryRe.FindStringSubmatch(trimmed); m != nil {
			lineEnd := findRustBlockEnd(lines, i)
			summary.Functions = append(summary.Functions, types.FuncInfo{
				Name:      m[4],
				LineStart: i + 1,
				LineEnd:   lineEnd,
			})
		}

		if m := rustStructSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Types = append(summary.Types, types.TypeInfo{
				Name: m[3],
				Kind: "struct",
				Line: i + 1,
			})
		}

		if m := rustEnumSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Types = append(summary.Types, types.TypeInfo{
				Name: m[3],
				Kind: "enum",
				Line: i + 1,
			})
		}

		if m := rustTraitSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Types = append(summary.Types, types.TypeInfo{
				Name: m[3],
				Kind: "trait",
				Line: i + 1,
			})
		}
	}
}

func findRustBlockEnd(lines []string, startIdx int) int {
	braceCount := 0
	foundOpen := false
	for i := startIdx; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				braceCount++
				foundOpen = true
			} else if ch == '}' {
				braceCount--
			}
		}
		if foundOpen && braceCount == 0 {
			return i + 1
		}
	}
	return len(lines)
}
