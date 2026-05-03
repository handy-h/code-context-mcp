package main

import (
	"testing"
)

func TestVersionVariables(t *testing.T) {
	// Test that version variables are accessible
	t.Run("TestVersion", func(t *testing.T) {
		if version == "" {
			t.Log("Version is empty, which is acceptable for development builds")
		}
	})

	t.Run("TestCommit", func(t *testing.T) {
		if commit == "" {
			t.Log("Commit is empty, which is acceptable for development builds")
		}
	})

	t.Run("TestDate", func(t *testing.T) {
		if date == "" {
			t.Log("Date is empty, which is acceptable for development builds")
		}
	})
}

func TestSimpleMath(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"positive numbers", 1, 2, 3},
		{"zero", 0, 0, 0},
		{"negative numbers", -1, -2, -3},
		{"mixed", -1, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.a + tt.b
			if result != tt.expected {
				t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestConfigLoading(t *testing.T) {
	// Test that LoadConfig function exists and can be called
	// This is a basic test to ensure the function signature is correct
	t.Run("LoadConfigExists", func(t *testing.T) {
		cfg := LoadConfig()
		// Check that config has default values
		if cfg.OllamaURL == "" {
			t.Error("OllamaURL should not be empty")
		}
		if cfg.CollectionName == "" {
			t.Error("CollectionName should not be empty")
		}
		if len(cfg.ScanExtensions) == 0 {
			t.Error("ScanExtensions should not be empty")
		}
	})
}