package file

import (
	"testing"
)

func TestExtractSummary_EmptyContent(t *testing.T) {
	summary := ExtractSummary("", "go", "test.go")
	if summary.Lines != 1 {
		t.Errorf("empty content Lines = %d, want 1", summary.Lines)
	}
	if summary.Language != "go" {
		t.Errorf("empty content Language = %q, want go", summary.Language)
	}
}

func TestExtractSummary_AutoDetect(t *testing.T) {
	content := "package main\n\nfunc main() {}"
	summary := ExtractSummary(content, "", "main.go")
	if summary.Language != "go" {
		t.Errorf("auto-detect Language = %q, want go", summary.Language)
	}
}

func TestExtractSummary_UnknownLang(t *testing.T) {
	content := "some content\nline2"
	summary := ExtractSummary(content, "unknown", "test.xyz")
	if summary.File != "test.xyz" {
		t.Errorf("File = %q, want test.xyz", summary.File)
	}
	if summary.Lines != 2 {
		t.Errorf("Lines = %d, want 2", summary.Lines)
	}
	if len(summary.Functions) != 0 {
		t.Errorf("unknown lang should have 0 functions, got %d", len(summary.Functions))
	}
}

func TestExtractSummary_Go(t *testing.T) {
	content := `package main

import "fmt"

func Hello() {
    fmt.Println("hello")
}

func (s *Server) Run() error {
    return nil
}

type Config struct {
    Name string
}

type Writer interface {
    Write([]byte) (int, error)
}`
	summary := ExtractSummary(content, "go", "main.go")

	if len(summary.Imports) == 0 {
		t.Error("Go summary should have imports")
	}

	if len(summary.Functions) < 2 {
		t.Errorf("Go summary should have >= 2 functions, got %d", len(summary.Functions))
	}

	if len(summary.Types) < 2 {
		t.Errorf("Go summary should have >= 2 types, got %d", len(summary.Types))
	}
}

func TestSummarizeGo_Imports(t *testing.T) {
	content := `package main

import "fmt"

func main() {}`
	summary := ExtractSummary(content, "go", "test.go")
	if len(summary.Imports) != 1 || summary.Imports[0] != "fmt" {
		t.Errorf("imports = %v, want [fmt]", summary.Imports)
	}
}

func TestSummarizeGo_ImportBlock(t *testing.T) {
	content := `package main

import (
    "fmt"
    "os"
    "strings"
)

func main() {}`
	summary := ExtractSummary(content, "go", "test.go")
	if len(summary.Imports) != 3 {
		t.Errorf("imports = %v, want 3 imports", summary.Imports)
	}
}

func TestSummarizeGo_Functions(t *testing.T) {
	content := `package main

func Hello() {
}

func World() {
}`
	summary := ExtractSummary(content, "go", "test.go")
	if len(summary.Functions) != 2 {
		t.Errorf("functions = %d, want 2", len(summary.Functions))
	}
	names := make(map[string]bool)
	for _, f := range summary.Functions {
		names[f.Name] = true
	}
	if !names["Hello"] || !names["World"] {
		t.Errorf("functions = %v, want Hello and World", names)
	}
}

func TestSummarizeGo_Structs(t *testing.T) {
	content := `package main

type Config struct {
    Name string
    Port int
}`
	summary := ExtractSummary(content, "go", "test.go")
	if len(summary.Types) != 1 {
		t.Errorf("types = %d, want 1", len(summary.Types))
	}
	if summary.Types[0].Name != "Config" {
		t.Errorf("type name = %q, want Config", summary.Types[0].Name)
	}
	if summary.Types[0].Kind != "struct" {
		t.Errorf("type kind = %q, want struct", summary.Types[0].Kind)
	}
}

func TestSummarizeGo_Interfaces(t *testing.T) {
	content := `package main

type Reader interface {
    Read([]byte) (int, error)
}`
	summary := ExtractSummary(content, "go", "test.go")
	if len(summary.Types) != 1 || summary.Types[0].Kind != "interface" {
		t.Errorf("types = %v, want 1 interface", summary.Types)
	}
}

func TestFindGoFuncEnd_Simple(t *testing.T) {
	lines := []string{"func Hello() {", "    fmt.Println()", "}"}
	end := findGoFuncEnd(lines, 0)
	if end != 3 {
		t.Errorf("findGoFuncEnd = %d, want 3", end)
	}
}

func TestFindGoFuncEnd_Nested(t *testing.T) {
	lines := []string{"func Hello() {", "    if true {", "    }", "}"}
	end := findGoFuncEnd(lines, 0)
	if end != 4 {
		t.Errorf("findGoFuncEnd = %d, want 4", end)
	}
}

func TestFindGoFuncEnd_Incomplete(t *testing.T) {
	lines := []string{"func Hello() {", "    fmt.Println()"}
	end := findGoFuncEnd(lines, 0)
	if end != len(lines) {
		t.Errorf("findGoFuncEnd(incomplete) = %d, want %d", end, len(lines))
	}
}

func TestExtractSummary_Vue(t *testing.T) {
	content := `<template><div>Hello</div></template>
<script>
export default {
    name: "App",
    methods: {
        hello() {}
    }
}
</script>`
	summary := ExtractSummary(content, "vue", "App.vue")
	if summary.Language != "vue" {
		t.Errorf("Language = %q, want vue", summary.Language)
	}
}

func TestExtractSummary_JS(t *testing.T) {
	content := `import React from "react";

function hello() {
    console.log("hello");
}

class App {
    render() {}
}`
	summary := ExtractSummary(content, "js", "app.js")
	if len(summary.Imports) == 0 {
		t.Error("JS summary should have imports")
	}
	if len(summary.Functions) == 0 {
		t.Error("JS summary should have functions")
	}
	if len(summary.Types) == 0 {
		t.Error("JS summary should have types (classes)")
	}
}

func TestExtractSummary_TS(t *testing.T) {
	content := `import { Component } from "@angular/core";

async function fetchData() {
    return await fetch("/api");
}`
	summary := ExtractSummary(content, "ts", "app.ts")
	if len(summary.Imports) == 0 {
		t.Error("TS summary should have imports")
	}
	if len(summary.Functions) == 0 {
		t.Error("TS summary should have functions")
	}
}

func TestExtractSummary_Markdown(t *testing.T) {
	content := `# Title

## Section 1

Some content.

## Section 2

More content.`
	summary := ExtractSummary(content, "md", "README.md")
	if len(summary.Functions) < 2 {
		t.Errorf("Markdown summary should have >= 2 sections, got %d", len(summary.Functions))
	}
}

func TestExtractSummary_Python(t *testing.T) {
	content := `import os

def hello():
    print("hello")

class Logger:
    def log(self, msg):
        print(msg)`
	summary := ExtractSummary(content, "py", "app.py")
	if len(summary.Imports) == 0 {
		t.Error("Python summary should have imports")
	}
	if len(summary.Functions) == 0 {
		t.Error("Python summary should have functions")
	}
	if len(summary.Types) == 0 {
		t.Error("Python summary should have types (classes)")
	}
}

func TestFindJSFuncEnd(t *testing.T) {
	lines := []string{"function hello() {", "    console.log()", "}"}
	end := findJSFuncEnd(lines, 0)
	if end != 3 {
		t.Errorf("findJSFuncEnd = %d, want 3", end)
	}
}

func TestFindPyFuncEnd_Basic(t *testing.T) {
	lines := []string{"def hello():", "    print('hello')", "    return True"}
	end := findPyFuncEnd(lines, 0)
	if end != 3 {
		t.Errorf("findPyFuncEnd = %d, want 3", end)
	}
}

func TestFindPyFuncEnd_EmptyBody(t *testing.T) {
	lines := []string{"def hello():", ""}
	end := findPyFuncEnd(lines, 0)
	if end != 2 {
		t.Errorf("findPyFuncEnd(empty body) = %d, want 2", end)
	}
}

func TestFindPyFuncEnd_BlankLines(t *testing.T) {
	lines := []string{"def hello():", "    x = 1", "", "    y = 2", "def world():"}
	end := findPyFuncEnd(lines, 0)
	if end != 4 {
		t.Errorf("findPyFuncEnd = %d, want 4", end)
	}
}
