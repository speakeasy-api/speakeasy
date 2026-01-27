package agent

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantShort string
		wantBody  string
	}{
		{
			name:      "valid frontmatter",
			input:     "---\nshort_description: \"Test description\"\nlong_description: |\n  A longer description.\n---\n\n# Title\n\nBody content here.\n",
			wantShort: "Test description",
			wantBody:  "# Title\n\nBody content here.\n",
		},
		{
			name:      "no frontmatter",
			input:     "# Just a title\n\nSome content.\n",
			wantShort: "",
			wantBody:  "# Just a title\n\nSome content.\n",
		},
		{
			name:      "frontmatter with BOM",
			input:     "\xef\xbb\xbf---\nshort_description: \"With BOM\"\n---\n\nContent after BOM.\n",
			wantShort: "With BOM",
			wantBody:  "Content after BOM.\n",
		},
		{
			name:      "empty file",
			input:     "",
			wantShort: "",
			wantBody:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := ParseFrontmatter(tt.input)

			if fm.ShortDescription != tt.wantShort {
				t.Errorf("ShortDescription = %q, want %q", fm.ShortDescription, tt.wantShort)
			}

			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "simple", input: "sdk-concepts", want: "sdk-concepts"},
		{name: "with md extension", input: "sdk-concepts.md", want: "sdk-concepts"},
		{name: "nested path", input: "sdk-testing/arazzo-testing", want: "sdk-testing/arazzo-testing"},
		{name: "nested with md", input: "sdk-testing/arazzo-testing.md", want: "sdk-testing/arazzo-testing"},
		{name: "backslashes", input: "sdk-testing\\arazzo-testing", want: "sdk-testing/arazzo-testing"},
		{name: "leading slash", input: "/sdk-concepts", want: "sdk-concepts"},
		{name: "trailing slash", input: "sdk-testing/", want: "sdk-testing"},
		{name: "dot", input: ".", want: ""},
		{name: "slash", input: "/", want: ""},
		{name: "double slash", input: "sdk-testing//arazzo-testing", want: "sdk-testing/arazzo-testing"},
		{name: "dot-dot rejected", input: "../etc/passwd", wantErr: true},
		{name: "nested dot-dot rejected", input: "sdk-testing/../../etc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizePath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	testFS := fstest.MapFS{
		"INDEX.md": &fstest.MapFile{
			Data: []byte("---\nshort_description: \"Index\"\n---\n\n# Index\n"),
		},
		"sdk-concepts.md": &fstest.MapFile{
			Data: []byte("---\nshort_description: \"SDK Concepts\"\n---\n\n# SDK Concepts\n"),
		},
		"sdk-testing/arazzo-testing.md": &fstest.MapFile{
			Data: []byte("---\nshort_description: \"Arazzo Testing\"\n---\n\n# Arazzo\n"),
		},
		"sdk-testing/contract-testing.md": &fstest.MapFile{
			Data: []byte("---\nshort_description: \"Contract Testing\"\n---\n\n# Contract\n"),
		},
	}

	tests := []struct {
		name         string
		path         string
		wantIsDir    bool
		wantResolved string
		wantErr      bool
	}{
		{
			name:         "empty path returns INDEX",
			path:         "",
			wantIsDir:    false,
			wantResolved: "INDEX.md",
		},
		{
			name:         "file without extension",
			path:         "sdk-concepts",
			wantIsDir:    false,
			wantResolved: "sdk-concepts.md",
		},
		{
			name:         "directory path",
			path:         "sdk-testing",
			wantIsDir:    true,
			wantResolved: "sdk-testing",
		},
		{
			name:         "nested file",
			path:         "sdk-testing/arazzo-testing",
			wantIsDir:    false,
			wantResolved: "sdk-testing/arazzo-testing.md",
		},
		{
			name:    "not found",
			path:    "nonexistent",
			wantErr: true,
		},
		{
			name:         "case insensitive fallback",
			path:         "SDK-Concepts",
			wantIsDir:    false,
			wantResolved: "sdk-concepts.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolvePath(testFS, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if result.IsDir != tt.wantIsDir {
				t.Errorf("ResolvePath(%q).IsDir = %v, want %v", tt.path, result.IsDir, tt.wantIsDir)
			}
			if result.ResolvedPath != tt.wantResolved {
				t.Errorf("ResolvePath(%q).ResolvedPath = %q, want %q", tt.path, result.ResolvedPath, tt.wantResolved)
			}
		})
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := Levenshtein(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGrepSearchesBodyOnly(t *testing.T) {
	testFS := fstest.MapFS{
		"test.md": &fstest.MapFile{
			Data: []byte("---\nshort_description: \"Hidden metadata\"\n---\n\n# Visible Title\n\nretryConfig:\n  strategy: backoff\n"),
		},
	}

	// Grep for content in body — should find it
	bodyMatches := grepFS(testFS, ".", "retryConfig", false)
	if len(bodyMatches) != 1 {
		t.Fatalf("expected 1 body match, got %d", len(bodyMatches))
	}
	if bodyMatches[0].Line != 3 {
		t.Errorf("expected line 3 (of body), got %d", bodyMatches[0].Line)
	}

	// Grep for content in frontmatter — should NOT find it
	fmMatches := grepFS(testFS, ".", "short_description", false)
	if len(fmMatches) != 0 {
		t.Errorf("expected 0 frontmatter matches, got %d", len(fmMatches))
	}

	// Grep for content that doesn't exist — should return empty
	noMatches := grepFS(testFS, ".", "nonexistent_string_xyz", false)
	if len(noMatches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(noMatches))
	}
}

func TestGrepWithContext(t *testing.T) {
	testFS := fstest.MapFS{
		"test.md": &fstest.MapFile{
			Data: []byte("---\nshort_description: \"Test\"\n---\n\nline1\nline2\nMATCH\nline4\nline5\n"),
		},
	}

	matches := grepFSWithContext(testFS, ".", "MATCH", false, 1, 1)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	m := matches[0]
	if len(m.ContextBefore) != 1 || m.ContextBefore[0] != "line2" {
		t.Errorf("ContextBefore = %v, want [line2]", m.ContextBefore)
	}
	if len(m.ContextAfter) != 1 || m.ContextAfter[0] != "line4" {
		t.Errorf("ContextAfter = %v, want [line4]", m.ContextAfter)
	}
}

func TestFindSuggestions(t *testing.T) {
	testFS := fstest.MapFS{
		"sdk-concepts.md":                &fstest.MapFile{Data: []byte("content")},
		"sdk-testing/arazzo-testing.md":   &fstest.MapFile{Data: []byte("content")},
		"sdk-testing/contract-testing.md": &fstest.MapFile{Data: []byte("content")},
	}

	suggestions := FindSuggestions(testFS, "sdk-cocepts") // typo
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions, got none")
	}
	// The closest match should be sdk-concepts
	if suggestions[0] != "sdk-concepts" {
		t.Errorf("first suggestion = %q, want %q", suggestions[0], "sdk-concepts")
	}
}

// grepFS is a test helper that performs a grep across an fs.FS.
func grepFS(contentFS fs.FS, root, pattern string, isRegex bool) []GrepMatch {
	return grepFSWithContext(contentFS, root, pattern, isRegex, 0, 0)
}

// grepFSWithContext performs grep with context lines.
func grepFSWithContext(contentFS fs.FS, root, pattern string, isRegex bool, before, after int) []GrepMatch {
	var filesToSearch []string
	fs.WalkDir(contentFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "." {
			return nil
		}
		filesToSearch = append(filesToSearch, p)
		return nil
	})

	matcher := func(line string) bool {
		return strings.Contains(line, pattern)
	}

	var allMatches []GrepMatch
	for _, filePath := range filesToSearch {
		data, err := fs.ReadFile(contentFS, filePath)
		if err != nil {
			continue
		}

		_, body := ParseFrontmatter(string(data))
		lines := strings.Split(body, "\n")

		for i, line := range lines {
			if !matcher(line) {
				continue
			}

			lineNum := i + 1
			var beforeCtx, afterCtx []string

			for b := max(0, i-before); b < i; b++ {
				beforeCtx = append(beforeCtx, lines[b])
			}
			for a := i + 1; a <= min(len(lines)-1, i+after); a++ {
				afterCtx = append(afterCtx, lines[a])
			}

			allMatches = append(allMatches, GrepMatch{
				File:          filePath,
				Line:          lineNum,
				Content:       line,
				ContextBefore: beforeCtx,
				ContextAfter:  afterCtx,
			})
		}
	}

	return allMatches
}
