package mcp

import (
	"strings"
	"testing"
)

func TestCompressText_OffPassthrough(t *testing.T) {
	in := "It is worth noting that this function returns the result."
	out, saved := compressText(in, "off")
	if out != in {
		t.Errorf("off mode should be passthrough, got %q", out)
	}
	if saved != 0 {
		t.Errorf("off mode should save 0%%, got %d", saved)
	}
}

func TestCompressText_LiteRemovesFiller(t *testing.T) {
	in := "Please note that the function basically returns the value."
	out, saved := compressText(in, "lite")

	if strings.Contains(strings.ToLower(out), "please note that") {
		t.Errorf("lite mode should strip 'please note that', got %q", out)
	}
	if strings.Contains(strings.ToLower(out), "basically") {
		t.Errorf("lite mode should strip 'basically', got %q", out)
	}
	if saved <= 0 {
		t.Errorf("expected positive savings, got %d", saved)
	}
}

func TestCompressText_FullRemovesArticles(t *testing.T) {
	in := "The function returns the result of the calculation."
	out, _ := compressText(in, "full")

	// "the" should be removed; original had 3 occurrences.
	if strings.Count(strings.ToLower(out), " the ") > 0 {
		t.Errorf("full mode should remove articles, got %q", out)
	}
}

func TestCompressText_UltraAbbreviates(t *testing.T) {
	in := "The configuration is saved without the documentation file."
	out, _ := compressText(in, "ultra")
	lower := strings.ToLower(out)

	if !strings.Contains(lower, "config") {
		t.Errorf("ultra should abbreviate 'configuration' to 'config', got %q", out)
	}
	if !strings.Contains(lower, "w/o") {
		t.Errorf("ultra should abbreviate 'without' to 'w/o', got %q", out)
	}
	if !strings.Contains(lower, "docs") {
		t.Errorf("ultra should abbreviate 'documentation' to 'docs', got %q", out)
	}
}

func TestCompressText_PreservesCodeBlocks(t *testing.T) {
	in := "The example below uses ```\nfunc main() {\n  fmt.Println(\"hello\")\n}\n``` to print."
	out, _ := compressText(in, "ultra")

	// Code block content must be untouched
	if !strings.Contains(out, `func main() {`) {
		t.Errorf("code block was mutated: %q", out)
	}
	if !strings.Contains(out, `fmt.Println("hello")`) {
		t.Errorf("code block content lost: %q", out)
	}
}

func TestCompressText_PreservesInlineCode(t *testing.T) {
	in := "The function `getUser()` returns the user."
	out, _ := compressText(in, "full")

	if !strings.Contains(out, "`getUser()`") {
		t.Errorf("inline code was mutated: %q", out)
	}
}

func TestCompressText_EmptyInput(t *testing.T) {
	out, saved := compressText("", "full")
	if out != "" {
		t.Errorf("empty input should return empty, got %q", out)
	}
	if saved != 0 {
		t.Errorf("empty input should save 0%%, got %d", saved)
	}
}

func TestCompressText_RealisticReduction(t *testing.T) {
	in := `Please note that it is important to note that the function basically
returns the result of the calculation. As a matter of fact, in order to
optimize this we should simply use a cache. Of course, that being said,
the implementation is quite straightforward.`

	out, saved := compressText(in, "full")
	if saved < 30 {
		t.Errorf("expected at least 30%% reduction on filler-heavy text, got %d%% (out: %q)", saved, out)
	}
}
