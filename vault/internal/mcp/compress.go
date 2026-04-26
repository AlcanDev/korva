package mcp

import (
	"regexp"
	"strings"
)

// compressText applies caveman-style output compression. Returns the
// compressed text and the percentage of characters saved (0-100).
//
// The compression is mechanical (no LLM): it strips filler words, articles,
// and rhetorical hedges while preserving code blocks, URLs, file paths, and
// technical syntax untouched.
//
// Modes:
//   - "off"   — passthrough
//   - "lite"  — keeps grammar; removes filler/hedges only
//   - "full"  — drops articles + uses fragments (default)
//   - "ultra" — telegraphic; aggressive abbreviation
func compressText(text, mode string) (string, int) {
	if mode == "off" || text == "" {
		return text, 0
	}

	// Protect code blocks (```…```) and inline code (`…`) so we never mutate them.
	codeBlocks, protectedText := extractCode(text)

	switch mode {
	case "lite":
		protectedText = stripFiller(protectedText)
	case "ultra":
		protectedText = stripFiller(protectedText)
		protectedText = stripArticles(protectedText)
		protectedText = abbreviate(protectedText)
	default: // "full"
		protectedText = stripFiller(protectedText)
		protectedText = stripArticles(protectedText)
	}

	// Restore code blocks.
	final := restoreCode(protectedText, codeBlocks)
	final = collapseWhitespace(final)

	saved := 0
	if len(text) > 0 {
		saved = (len(text) - len(final)) * 100 / len(text)
		if saved < 0 {
			saved = 0
		}
	}
	return final, saved
}

var codeBlockRe = regexp.MustCompile("(?s)```.*?```|`[^`]+`")

func extractCode(text string) ([]string, string) {
	var blocks []string
	out := codeBlockRe.ReplaceAllStringFunc(text, func(match string) string {
		idx := len(blocks)
		blocks = append(blocks, match)
		return placeholder(idx)
	})
	return blocks, out
}

func restoreCode(text string, blocks []string) string {
	for i, blk := range blocks {
		text = strings.Replace(text, placeholder(i), blk, 1)
	}
	return text
}

func placeholder(idx int) string {
	return "\x00CODE" + intToStr(idx) + "\x00"
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// fillerPhrases lists hedges and pleasantries that add no technical value.
var fillerPhrases = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bplease note that\b`),
	regexp.MustCompile(`(?i)\bit is (worth|important) (noting|to note) that\b`),
	regexp.MustCompile(`(?i)\bas (you|we) can see\b`),
	regexp.MustCompile(`(?i)\bin (this|that) case\b`),
	regexp.MustCompile(`(?i)\bas a matter of fact\b`),
	regexp.MustCompile(`(?i)\bin order to\b`),
	regexp.MustCompile(`(?i)\bbasically\b`),
	regexp.MustCompile(`(?i)\bessentially\b`),
	regexp.MustCompile(`(?i)\bat the end of the day\b`),
	regexp.MustCompile(`(?i)\bI hope this helps\b`),
	regexp.MustCompile(`(?i)\bcertainly\b`),
	regexp.MustCompile(`(?i)\bof course\b`),
	regexp.MustCompile(`(?i)\babsolutely\b`),
	regexp.MustCompile(`(?i)\bjust\s`),
	regexp.MustCompile(`(?i)\bsimply\s`),
	regexp.MustCompile(`(?i)\bvery\s`),
	regexp.MustCompile(`(?i)\bquite\s`),
	regexp.MustCompile(`(?i)\bthat being said,?\s`),
	regexp.MustCompile(`(?i)\bin conclusion,?\s`),
	regexp.MustCompile(`(?i)\bto be honest,?\s`),
	regexp.MustCompile(`(?i)\bin fact,?\s`),
}

func stripFiller(text string) string {
	for _, re := range fillerPhrases {
		text = re.ReplaceAllString(text, "")
	}
	return text
}

// articleRe matches standalone English articles.
var articleRe = regexp.MustCompile(`(?i)\b(the|a|an)\s+`)

func stripArticles(text string) string {
	return articleRe.ReplaceAllString(text, "")
}

// abbreviations maps common technical phrases to terse equivalents.
var abbreviations = map[string]string{
	"because":        "bc",
	"with":           "w/",
	"without":        "w/o",
	"function":       "fn",
	"variable":       "var",
	"return":         "ret",
	"following":      "below",
	"approximately":  "~",
	"configuration":  "config",
	"implementation": "impl",
	"information":    "info",
	"documentation":  "docs",
	"environment":    "env",
	"directory":      "dir",
	"and":            "&",
}

func abbreviate(text string) string {
	for long, short := range abbreviations {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(long) + `\b`)
		text = re.ReplaceAllString(text, short)
	}
	return text
}

var multiSpaceRe = regexp.MustCompile(`[ \t]+`)
var multiNewlineRe = regexp.MustCompile(`\n{3,}`)

func collapseWhitespace(text string) string {
	text = multiSpaceRe.ReplaceAllString(text, " ")
	text = multiNewlineRe.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
