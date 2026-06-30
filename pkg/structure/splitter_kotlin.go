package structure

import (
	"regexp"
	"strings"

	"github.com/handy-h/code-context-mcp/internal/types"
)

// ================= Kotlin 切分策略 =================
// 策略：按 fun/class/object/interface/val/var/typealias/annotation 边界切分
// import/package 归入 header 块

var (
	// 函数：支持 visibility、suspend、inline、tailrec、operator、infix、external、override、expect、actual 修饰符
	// 匹配: fun name( 或 suspend fun name( 等；支持泛型参数 fun <T> name(
	kotlinFuncRe = regexp.MustCompile(`^\s*(?:(?:public|private|internal|protected|open|override|expect|actual)\s+)?(?:(?:suspend|inline|tailrec|operator|infix|external)\s+)*fun\s+(?:<[^>]+>\s*)?(\w+)`)

	// class: 匹配 class/data class/sealed class/open class/abstract class/inner class
	// 支持 expect/actual (Kotlin Multiplatform)
	kotlinClassRe = regexp.MustCompile(`^\s*(?:(?:public|private|internal|protected|open|abstract)\s+)?(?:expect|actual)?\s*(?:data\s+)?(?:sealed\s+)?(?:abstract\s+)?(?:open\s+)?(?:inner\s+)?class\s+(\w+)`)

	// enum class (单独处理)
	kotlinEnumClassRe = regexp.MustCompile(`^\s*enum\s+class\s+(\w+)`)

	// annotation class
	kotlinAnnotationClassRe = regexp.MustCompile(`^\s*annotation\s+class\s+(\w+)`)

	// value class / inline class
	kotlinValueClassRe = regexp.MustCompile(`^\s*(?:value|inline)\s+class\s+(\w+)`)

	// interface (支持 expect/actual)
	kotlinInterfaceRe = regexp.MustCompile(`^\s*(?:(?:public|private|internal|protected)\s+)?(?:expect|actual)?\s*interface\s+(\w+)`)

	// object / companion object / data object
	kotlinObjectRe    = regexp.MustCompile(`^\s*(?:data\s+)?object\s+(\w+)`)
	kotlinCompanionRe = regexp.MustCompile(`^\s*companion\s+object(?:\s+(\w+))?`)

	// typealias
	kotlinTypealiasRe = regexp.MustCompile(`^\s*typealias\s+(\w+)`)

	// 顶层 val/var（不在函数/类内部）
	kotlinValVarRe = regexp.MustCompile(`^\s*(?:(?:private|internal|public|protected)\s+)?(?:const\s+)?(?:val|var)\s+(\w+)`)

	// import / package 行
	kotlinImportRe = regexp.MustCompile(`^\s*(?:package|import)\s+`)
)

func chunkKotlin(content string, filePath string) []types.CodeChunk {
	lines := strings.Split(content, "\n")
	type boundary struct {
		lineIdx int
		symbol  string
		kind    string
	}

	var boundaries []boundary
	importEndLine := 0

	// 括号深度跟踪（圆括号+大括号），用于跳过类构造函数参数和函数内部的 val/var
	var parenDepth int
	var braceDepth int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 跳过空行和注释（注释行不跟踪括号，避免注释内的括号影响深度判断）
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") {
			continue
		}

		// 计算本行的括号变化
		lineParenDelta := strings.Count(trimmed, "(") - strings.Count(trimmed, ")")
		lineBraceDelta := strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

		// import/package 行：仅顶层计入 header
		if kotlinImportRe.MatchString(trimmed) && parenDepth == 0 && braceDepth == 0 {
			importEndLine = i + 1
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// companion object（含无名，先于 object 匹配）
		if m := kotlinCompanionRe.FindStringSubmatch(trimmed); m != nil {
			symbol := "Companion"
			if m[1] != "" {
				symbol = m[1]
			}
			boundaries = append(boundaries, boundary{i, symbol, "companion_object"})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// data object / object (non-companion)
		if m := kotlinObjectRe.FindStringSubmatch(trimmed); m != nil && !strings.Contains(trimmed, "companion") {
			kind := "object"
			if strings.Contains(trimmed, "data") {
				kind = "data_object"
			}
			boundaries = append(boundaries, boundary{i, m[1], kind})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// enum class
		if m := kotlinEnumClassRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "enum"})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// annotation class
		if m := kotlinAnnotationClassRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "annotation"})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// value class / inline class
		if m := kotlinValueClassRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "value_class"})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// interface
		if m := kotlinInterfaceRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "interface"})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// class/data class/sealed class/expect class/actual class 等
		if m := kotlinClassRe.FindStringSubmatch(trimmed); m != nil {
			kind := determineClassKind(trimmed)
			boundaries = append(boundaries, boundary{i, m[1], kind})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// typealias（仅顶层）
		if m := kotlinTypealiasRe.FindStringSubmatch(trimmed); m != nil {
			if parenDepth == 0 && braceDepth == 0 {
				boundaries = append(boundaries, boundary{i, m[1], "typealias"})
			}
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// fun（函数/方法）
		if m := kotlinFuncRe.FindStringSubmatch(trimmed); m != nil {
			boundaries = append(boundaries, boundary{i, m[1], "function"})
			updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
			continue
		}

		// 顶层 val/var（不在构造函数参数或函数/类内部）
		if strings.Contains(trimmed, "val ") || strings.Contains(trimmed, "var ") {
			if m := kotlinValVarRe.FindStringSubmatch(trimmed); m != nil {
				if parenDepth == 0 && braceDepth == 0 {
					boundaries = append(boundaries, boundary{i, m[1], "property"})
				}
			}
		}

		// 统一更新括号深度
		updateDepths(&parenDepth, &braceDepth, lineParenDelta, lineBraceDelta)
	}

	// 如果没有找到任何边界且没有header，返回 nil 让上层做整文件切分
	if len(boundaries) == 0 && importEndLine == 0 {
		return nil
	}

	var chunks []types.CodeChunk
	firstBoundary := len(lines)
	if len(boundaries) > 0 {
		firstBoundary = boundaries[0].lineIdx
	}

	// header 块：package/import 之后、第一个结构之前
	headerEnd := importEndLine
	if firstBoundary > headerEnd {
		headerEnd = firstBoundary
	}
	if headerEnd > 0 {
		headerContent := strings.Join(lines[:headerEnd], "\n")
		if strings.TrimSpace(headerContent) != "" {
			chunks = append(chunks, types.CodeChunk{
				Content: headerContent,
				Metadata: map[string]interface{}{
					"file":       filePath,
					"symbol":     "header",
					"line_start": 1,
					"line_end":   headerEnd,
					"type":       "header",
					"language":   "kotlin",
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
				"language":   "kotlin",
			},
		})
	}

	return chunks
}

// updateDepths 统一更新括号深度（clamp 到 0 防止负值）
func updateDepths(paren, brace *int, parenDelta, braceDelta int) {
	*paren = max(*paren+parenDelta, 0)
	*brace = max(*brace+braceDelta, 0)
}

// determineClassKind 根据行内修饰符判断类型
func determineClassKind(line string) string {
	if strings.Contains(line, "data") {
		return "data_class"
	}
	if strings.Contains(line, "sealed") {
		return "sealed_class"
	}
	return "class"
}
