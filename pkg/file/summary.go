package file

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
	"github.com/handy-h/code-context-mcp/pkg/structure"
)

// ExtractSummary 提取文件结构摘要
func ExtractSummary(content string, lang string, filePath string) *types.FileSummary {
	lines := strings.Split(content, "\n")
	if lang == "" {
		lang = structure.DetectLanguage(filePath)
	}

	summary := &types.FileSummary{
		File:     filePath,
		Lines:    len(lines),
		Language: lang,
	}

	switch lang {
	case "go":
		extractGoSummary(content, summary)
	case "vue":
		extractVueSummary(content, summary)
	case "js", "ts":
		extractJSTSSummary(content, summary, lang)
	case "md":
		extractMarkdownSummary(content, summary)
	case "py":
		extractPythonSummary(content, summary)
	default:
		// 仅返回基本信息
	}

	return summary
}

// ================= Go 摘要提取 =================

var (
	goImportSingleRe     = regexp.MustCompile(`^import\s+"([^"]+)"`)
	goImportGroupRe      = regexp.MustCompile(`"([^"]+)"`)
	goFuncSummaryRe      = regexp.MustCompile(`^func\s+(\(\w+\s+\*?\w+\)\s+)?(\w+)`)
	goStructSummaryRe    = regexp.MustCompile(`^type\s+(\w+)\s+struct`)
	goInterfaceSummaryRe = regexp.MustCompile(`^type\s+(\w+)\s+interface`)
)

func extractGoSummary(content string, summary *types.FileSummary) {
	lines := strings.Split(content, "\n")
	inImportBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 提取导入
		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
				continue
			}
			if m := goImportGroupRe.FindStringSubmatch(trimmed); m != nil {
				summary.Imports = append(summary.Imports, m[1])
			}
			continue
		}
		if m := goImportSingleRe.FindStringSubmatch(trimmed); m != nil {
			summary.Imports = append(summary.Imports, m[1])
		}

		// 提取函数
		if m := goFuncSummaryRe.FindStringSubmatch(trimmed); m != nil {
			funcName := m[2]
			lineEnd := findGoFuncEnd(lines, i)
			summary.Functions = append(summary.Functions, types.FuncInfo{
				Name:      funcName,
				LineStart: i + 1,
				LineEnd:   lineEnd,
			})
		}

		// 提取类型
		if m := goStructSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Types = append(summary.Types, types.TypeInfo{
				Name: m[1],
				Kind: "struct",
				Line: i + 1,
			})
		}
		if m := goInterfaceSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Types = append(summary.Types, types.TypeInfo{
				Name: m[1],
				Kind: "interface",
				Line: i + 1,
			})
		}
	}
}

// findGoFuncEnd 估算 Go 函数结束行（基于大括号匹配）
func findGoFuncEnd(lines []string, startIdx int) int {
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

// ================= Vue 摘要提取 =================

func extractVueSummary(content string, summary *types.FileSummary) {
	// 提取 script 部分的摘要
	scriptContent := structure.ExtractScriptContent(content)
	if scriptContent != "" {
		extractJSTSSummary(scriptContent, summary, "js")
	}
}

// ================= JS/TS 摘要提取 =================

var (
	jsImportRe       = regexp.MustCompile(`^import\s+.+\s+from\s+['"]([^'"]+)['"]`)
	jsFuncSummaryRe  = regexp.MustCompile(`(async\s+)?function\s+(\w+)`)
	jsArrowFuncRe    = regexp.MustCompile(`(const|let|var)\s+(\w+)\s*=\s*(async\s+)?\(`)
	jsClassSummaryRe = regexp.MustCompile(`class\s+(\w+)`)
)

func extractJSTSSummary(content string, summary *types.FileSummary, lang string) {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 提取导入
		if m := jsImportRe.FindStringSubmatch(trimmed); m != nil {
			summary.Imports = append(summary.Imports, m[1])
		}

		// 提取函数
		if m := jsFuncSummaryRe.FindStringSubmatch(trimmed); m != nil {
			lineEnd := findJSFuncEnd(lines, i)
			summary.Functions = append(summary.Functions, types.FuncInfo{
				Name:      m[2],
				LineStart: i + 1,
				LineEnd:   lineEnd,
			})
		} else if m := jsArrowFuncRe.FindStringSubmatch(trimmed); m != nil {
			lineEnd := findJSFuncEnd(lines, i)
			summary.Functions = append(summary.Functions, types.FuncInfo{
				Name:      m[2],
				LineStart: i + 1,
				LineEnd:   lineEnd,
			})
		}

		// 提取类
		if m := jsClassSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Types = append(summary.Types, types.TypeInfo{
				Name: m[1],
				Kind: "class",
				Line: i + 1,
			})
		}
	}
}

// findJSFuncEnd 估算 JS/TS 函数结束行
func findJSFuncEnd(lines []string, startIdx int) int {
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

// ================= Markdown 摘要提取 =================

var mdHeadingSummaryRe = regexp.MustCompile(`^(#{1,6})\s+(.+)`)

func extractMarkdownSummary(content string, summary *types.FileSummary) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if m := mdHeadingSummaryRe.FindStringSubmatch(line); m != nil {
			// 找到下一个同级或更高级标题
			level := len(m[1])
			lineEnd := len(lines)
			for j := i + 1; j < len(lines); j++ {
				if m2 := mdHeadingSummaryRe.FindStringSubmatch(lines[j]); m2 != nil {
					if len(m2[1]) <= level {
						lineEnd = j
						break
					}
				}
			}
			summary.Functions = append(summary.Functions, types.FuncInfo{
				Name:      strings.TrimSpace(m[2]),
				LineStart: i + 1,
				LineEnd:   lineEnd,
			})
		}
	}
}

// ================= Python 摘要提取 =================

var (
	pyImportRe       = regexp.MustCompile(`^(import|from)\s+(\w+)`)
	pyFuncSummaryRe  = regexp.MustCompile(`^(async\s+)?def\s+(\w+)`)
	pyClassSummaryRe = regexp.MustCompile(`^class\s+(\w+)`)
)

func extractPythonSummary(content string, summary *types.FileSummary) {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if m := pyImportRe.FindStringSubmatch(trimmed); m != nil {
			summary.Imports = append(summary.Imports, m[2])
		}

		// 提取函数
		if m := pyFuncSummaryRe.FindStringSubmatch(trimmed); m != nil {
			lineEnd := findPyFuncEnd(lines, i)
			summary.Functions = append(summary.Functions, types.FuncInfo{
				Name:      m[2],
				LineStart: i + 1,
				LineEnd:   lineEnd,
			})
		}

		// 提取类
		if m := pyClassSummaryRe.FindStringSubmatch(trimmed); m != nil {
			summary.Types = append(summary.Types, types.TypeInfo{
				Name: m[1],
				Kind: "class",
				Line: i + 1,
			})
		}
	}
}

// findPyFuncEnd 估算 Python 函数结束行（基于缩进）
func findPyFuncEnd(lines []string, startIdx int) int {
	if startIdx+1 >= len(lines) {
		return startIdx + 1
	}
	// 获取函数体的缩进级别
	defIndent := len(lines[startIdx]) - len(strings.TrimLeft(lines[startIdx], " \t"))
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		currentIndent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
		if currentIndent <= defIndent {
			return i
		}
	}
	return len(lines)
}
