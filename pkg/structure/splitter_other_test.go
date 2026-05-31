package structure

import (
	"testing"
)

func TestChunkJSTS_ExportFunc(t *testing.T) {
	content := `export function hello() {
    console.log("hello");
}`
	chunks := chunkJSTS(content, "app.js", "js")
	if len(chunks) == 0 {
		t.Fatal("chunkJSTS returned empty for export function")
	}
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "hello" {
			found = true
		}
	}
	if !found {
		t.Error("hello function not found in chunks")
	}
}

func TestChunkJSTS_ExportDefaultFunc(t *testing.T) {
	content := `export default function init() {
    return true;
}`
	chunks := chunkJSTS(content, "app.js", "js")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "init" {
			found = true
		}
	}
	if !found {
		t.Error("init function not found in chunks")
	}
}

func TestChunkJSTS_ExportClass(t *testing.T) {
	content := `export class UserService {
    constructor() {}
}`
	chunks := chunkJSTS(content, "service.ts", "ts")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "UserService" {
			found = true
		}
	}
	if !found {
		t.Error("UserService class not found in chunks")
	}
}

func TestChunkJSTS_ExportConst(t *testing.T) {
	content := `export const MAX_RETRIES = 3;`
	chunks := chunkJSTS(content, "config.js", "js")
	if len(chunks) == 0 {
		t.Fatal("chunkJSTS returned empty for export const")
	}
}

func TestChunkJSTS_Function(t *testing.T) {
	content := `function calculate(a, b) {
    return a + b;
}`
	chunks := chunkJSTS(content, "math.js", "js")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "calculate" {
			found = true
		}
	}
	if !found {
		t.Error("calculate function not found")
	}
}

func TestChunkJSTS_Class(t *testing.T) {
	content := `class Logger {
    log(msg) {}
}`
	chunks := chunkJSTS(content, "logger.js", "js")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "Logger" {
			found = true
		}
	}
	if !found {
		t.Error("Logger class not found")
	}
}

func TestChunkJSTS_Const(t *testing.T) {
	content := `const API_URL = "https://api.example.com";`
	chunks := chunkJSTS(content, "config.js", "js")
	if len(chunks) == 0 {
		t.Fatal("chunkJSTS returned empty for const")
	}
}

func TestChunkJSTS_AsyncFunc(t *testing.T) {
	content := `async function fetchData() {
    return await fetch("/api");
}`
	chunks := chunkJSTS(content, "api.js", "js")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "fetchData" {
			found = true
		}
	}
	if !found {
		t.Error("async fetchData not found")
	}
}

func TestChunkJSTS_WithHeader(t *testing.T) {
	content := `import React from "react";

export function App() {
    return <div />;
}`
	chunks := chunkJSTS(content, "App.jsx", "js")
	if len(chunks) < 2 {
		t.Errorf("chunkJSTS should find header + function, got %d chunks", len(chunks))
	}
}

func TestChunkJSTS_NoBoundary(t *testing.T) {
	content := `// just a comment
x = 1;`
	chunks := chunkJSTS(content, "test.js", "js")
	if len(chunks) != 0 {
		t.Errorf("chunkJSTS(no boundary) = %d chunks, want 0", len(chunks))
	}
}

func TestChunkJSTS_LangParam(t *testing.T) {
	content := `function hello() {}`
	chunks := chunkJSTS(content, "app.ts", "ts")
	for _, c := range chunks {
		if c.Metadata["language"] != "ts" {
			t.Errorf("language = %v, want ts", c.Metadata["language"])
		}
	}
}

func TestChunkMarkdown_H1H2H3(t *testing.T) {
	content := `# Title

Some intro text.

## Section 1

Content here.

### Subsection

More content.

## Section 2

Final content.`
	chunks := chunkMarkdown(content, "README.md")
	if len(chunks) < 3 {
		t.Errorf("chunkMarkdown should find >= 3 sections, got %d", len(chunks))
	}
	symbols := make(map[string]bool)
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok {
			symbols[sym] = true
		}
	}
	if !symbols["Title"] {
		t.Error("Title heading not found")
	}
	if !symbols["Section 1"] {
		t.Error("Section 1 heading not found")
	}
}

func TestChunkMarkdown_Preamble(t *testing.T) {
	content := `Some preamble text.

## First Heading

Content.`
	chunks := chunkMarkdown(content, "README.md")
	if len(chunks) == 0 {
		t.Fatal("chunkMarkdown returned empty")
	}
}

func TestChunkMarkdown_SingleH1(t *testing.T) {
	content := `# My Document

All content here.`
	chunks := chunkMarkdown(content, "README.md")
	if len(chunks) == 0 {
		t.Fatal("chunkMarkdown returned empty for single H1")
	}
}

func TestChunkMarkdown_NoHeadings(t *testing.T) {
	content := `Just some text
without any headings.`
	chunks := chunkMarkdown(content, "README.md")
	if len(chunks) != 0 {
		t.Errorf("chunkMarkdown(no headings) = %d, want 0", len(chunks))
	}
}

func TestChunkPython_Def(t *testing.T) {
	content := `def hello():
    print("hello")`
	chunks := chunkPython(content, "main.py")
	if len(chunks) == 0 {
		t.Fatal("chunkPython returned empty for def")
	}
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "hello" {
			found = true
		}
	}
	if !found {
		t.Error("hello function not found")
	}
}

func TestChunkPython_AsyncDef(t *testing.T) {
	content := `async def fetch_data():
    return await get("/api")`
	chunks := chunkPython(content, "api.py")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "fetch_data" {
			found = true
		}
	}
	if !found {
		t.Error("async fetch_data not found")
	}
}

func TestChunkPython_Class(t *testing.T) {
	content := `class UserService:
    def __init__(self):
        pass`
	chunks := chunkPython(content, "service.py")
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "UserService" {
			found = true
		}
	}
	if !found {
		t.Error("UserService class not found")
	}
}

func TestChunkPython_Mixed(t *testing.T) {
	content := `import os

def hello():
    pass

class Logger:
    def log(self):
        pass`
	chunks := chunkPython(content, "app.py")
	if len(chunks) < 3 {
		t.Errorf("chunkPython should find >= 3 chunks, got %d", len(chunks))
	}
}

func TestChunkPython_Header(t *testing.T) {
	content := `import os
import sys

def hello():
    pass`
	chunks := chunkPython(content, "app.py")
	if len(chunks) < 2 {
		t.Errorf("chunkPython should find header + def, got %d", len(chunks))
	}
}

func TestChunkPython_NoBoundary(t *testing.T) {
	content := `# just a comment
x = 1`
	chunks := chunkPython(content, "test.py")
	if len(chunks) != 0 {
		t.Errorf("chunkPython(no boundary) = %d, want 0", len(chunks))
	}
}

func TestExtractScriptContent_Normal(t *testing.T) {
	content := `<template><div>Hello</div></template>
<script>
export default {
    name: "App"
}
</script>`
	script := ExtractScriptContent(content)
	if script == "" {
		t.Fatal("ExtractScriptContent returned empty for normal SFC")
	}
	if !contains(script, "export default") {
		t.Error("script content should contain export default")
	}
}

func TestExtractScriptContent_WithScriptTagInString(t *testing.T) {
	content := `<script>
const closing = "</" + "script>";
export default {}
</script>`
	script := ExtractScriptContent(content)
	if script == "" {
		t.Fatal("ExtractScriptContent returned empty")
	}
}

func TestExtractScriptContent_NoScript(t *testing.T) {
	content := `<template><div>Hello</div></template>`
	script := ExtractScriptContent(content)
	if script != "" {
		t.Errorf("ExtractScriptContent(no script) = %q, want empty", script)
	}
}

func TestExtractScriptContent_CaseInsensitive(t *testing.T) {
	content := `<SCRIPT>
const x = 1;
</SCRIPT>`
	script := ExtractScriptContent(content)
	if script == "" {
		t.Error("ExtractScriptContent should be case-insensitive")
	}
}

func TestExtractScriptContent_WithAttributes(t *testing.T) {
	content := `<script lang="ts" setup>
const x = 1;
</script>`
	script := ExtractScriptContent(content)
	if script == "" {
		t.Error("ExtractScriptContent should handle script attributes")
	}
	if !contains(script, "const x = 1") {
		t.Error("script content should include the actual code")
	}
}

func TestChunkVue_TemplateScriptStyle(t *testing.T) {
	content := `<template>
<div>Hello</div>
</template>
<script>
export default {
    name: "App"
}
</script>
<style>
.app { color: red; }
</style>`
	chunks := chunkVue(content, "App.vue")
	if len(chunks) == 0 {
		t.Fatal("chunkVue returned empty for full SFC")
	}
	found := false
	for _, c := range chunks {
		if sym, ok := c.Metadata["symbol"].(string); ok && sym == "App" {
			found = true
		}
	}
	if !found {
		t.Error("App chunk not found")
	}
}

func TestChunkVue_ScriptOnly(t *testing.T) {
	content := `<script>
function hello() {}
</script>`
	chunks := chunkVue(content, "Hello.vue")
	if len(chunks) == 0 {
		t.Fatal("chunkVue returned empty for script-only SFC")
	}
}

func TestChunkVue_NoBlocks(t *testing.T) {
	content := `just some text without any blocks`
	chunks := chunkVue(content, "plain.vue")
	if len(chunks) != 0 {
		t.Errorf("chunkVue(no blocks) = %d, want 0", len(chunks))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
