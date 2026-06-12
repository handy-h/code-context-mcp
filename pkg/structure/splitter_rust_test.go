package structure

import (
	"testing"
)

func TestChunkRust(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantTypes []string
	}{
		{
			name: "简单函数",
			input: `fn main() {
    println!("Hello");
}`,
			wantLen:   1,
			wantTypes: []string{"function"},
		},
		{
			name: "结构体和实现",
			input: `struct Point {
    x: f64,
    y: f64,
}

impl Point {
    fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }
}`,
			wantLen:   3,
			wantTypes: []string{"struct", "impl", "function"},
		},
		{
			name: "带header的完整文件",
			input: `use std::io;

mod utils;

pub struct Config {
    pub name: String,
}

pub trait Processable {
    fn process(&self) -> Result<(), Error>;
}

impl Processable for Config {
    fn process(&self) -> Result<(), Error> {
        Ok(())
    }
}

fn main() {
    let config = Config { name: "test".into() };
    config.process().unwrap();
}`,
			wantLen:   8,
			wantTypes: []string{"header", "module", "struct", "trait", "function", "impl", "function", "function"},
		},
		{
			name:     "空内容",
			input:    "",
			wantLen:  0,
			wantTypes: nil,
		},
		{
			name: "枚举和宏",
			input: `enum Color {
    Red,
    Green,
    Blue,
}

macro_rules! say_hello {
    () => {
        println!("Hello!");
    };
}`,
			wantLen:   2,
			wantTypes: []string{"enum", "macro"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := chunkRust(tt.input, "test.rs")
			if len(chunks) != tt.wantLen {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.wantLen)
				for i, c := range chunks {
					t.Logf("  chunk[%d]: type=%s symbol=%s", i, c.Metadata["type"], c.Metadata["symbol"])
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
			}
		})
	}
}

func TestDetectLanguageRust(t *testing.T) {
	if lang := DetectLanguage("main.rs"); lang != "rust" {
		t.Errorf("expected 'rust', got '%s'", lang)
	}
	if lang := DetectLanguage("lib.rs"); lang != "rust" {
		t.Errorf("expected 'rust', got '%s'", lang)
	}
}
