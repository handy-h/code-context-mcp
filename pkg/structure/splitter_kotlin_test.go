package structure

import (
	"testing"
)

func TestChunkKotlin(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantTypes []string
	}{
		{
			name: "简单函数",
			input: `fun main() {
    println("Hello, Kotlin!")
}`,
			wantLen:   1,
			wantTypes: []string{"function"},
		},
		{
			name: "数据类和函数",
			input: `data class Person(
    val name: String,
    val age: Int
)

fun greet(person: Person): String {
    return "Hello, ${person.name}!"
}`,
			wantLen:   2,
			wantTypes: []string{"data_class", "function"},
		},
		{
			name: "带header的完整文件",
			input: `package com.example.app

import kotlinx.coroutines.*

class MyService(private val repo: Repository) {

    fun doSomething(): Result {
        return repo.fetch()
    }

    companion object {
        const val TAG = "MyService"
    }
}

interface Repository {
    suspend fun fetch(): Result
}

object Config {
    val baseUrl: String = "https://api.example.com"
}

private fun helper() {
    // utility
}`,
			wantLen:   8,
			wantTypes: []string{"header", "class", "function", "companion_object", "interface", "function", "object", "function"},
		},
		{
			name: "枚举和注解",
			input: `enum class Color(val rgb: Int) {
    RED(0xFF0000),
    GREEN(0x00FF00),
    BLUE(0x0000FF)
}

annotation class Fancy

typealias NameMap = Map<String, String>`,

			wantLen:   3,
			wantTypes: []string{"enum", "annotation", "typealias"},
		},
		{
			name: "密封类和值类",
			input: `sealed class Result<out T> {
    data class Success<T>(val data: T) : Result<T>()
    data class Error(val exception: Exception) : Result<Nothing>()
}

value class UserId(val id: String)

fun handle(result: Result<UserId>) {
    when (result) {
        is Result.Success -> println(result.data)
        is Result.Error -> println(result.exception)
    }
}`,
			wantLen:   5,
			wantTypes: []string{"sealed_class", "data_class", "data_class", "value_class", "function"},
		},
		{
			name: "顶层属性和扩展函数",
			input: `const val MAX_SIZE = 100

val defaultConfig = mapOf(
    "key" to "value"
)

fun String.addExclamation(): String = this + "!"

fun main() {
    println("hello".addExclamation())
}`,
			wantLen:   4,
			wantTypes: []string{"property", "property", "function", "function"},
		},
		{
			name:     "空内容",
			input:    "",
			wantLen:  0,
			wantTypes: nil,
		},
		{
			name: "纯import文件",
			input: `package com.example

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity`,
			wantLen:   1,
			wantTypes: []string{"header"},
		},
		{
			name: "suspend函数和内联函数",
			input: `suspend fun fetchData(): List<Item> {
    delay(1000)
    return emptyList()
}

inline fun <reified T> logger(): Logger = LoggerFactory.getLogger(T::class.java)

private inline fun measureTime(block: () -> Unit): Long {
    val start = System.currentTimeMillis()
    block()
    return System.currentTimeMillis() - start
}`,
			wantLen:   3,
			wantTypes: []string{"function", "function", "function"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := chunkKotlin(tt.input, "test.kt")
			if len(chunks) != tt.wantLen {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.wantLen)
				for i, c := range chunks {
					t.Logf("  chunk[%d]: type=%s symbol=%s lines=%v-%v",
						i, c.Metadata["type"], c.Metadata["symbol"],
						c.Metadata["line_start"], c.Metadata["line_end"])
					t.Logf("  content: %s", truncateForLog(c.Content, 80))
				}
				return
			}
			for i, c := range chunks {
				if i < len(tt.wantTypes) {
					gotType, _ := c.Metadata["type"].(string)
					if gotType != tt.wantTypes[i] {
						t.Errorf("chunk[%d] type = %s, want %s", i, gotType, tt.wantTypes[i])
					}
				}
				// 验证必要字段存在
				if c.Metadata["symbol"] == nil || c.Metadata["symbol"] == "" {
					t.Errorf("chunk[%d] missing symbol", i)
				}
				if c.Metadata["line_start"] == nil {
					t.Errorf("chunk[%d] missing line_start", i)
				}
			}
		})
	}
}

func TestDetectLanguageKotlin(t *testing.T) {
	if lang := DetectLanguage("Main.kt"); lang != "kotlin" {
		t.Errorf("expected 'kotlin', got '%s'", lang)
	}
	if lang := DetectLanguage("Utils.kts"); lang != "kotlin" {
		t.Errorf("expected 'kotlin', got '%s'", lang)
	}
	if lang := DetectLanguage("src/main/kotlin/App.kt"); lang != "kotlin" {
		t.Errorf("expected 'kotlin', got '%s'", lang)
	}
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
