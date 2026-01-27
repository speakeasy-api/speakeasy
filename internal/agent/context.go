package agent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter represents YAML frontmatter from agent-context markdown files.
type Frontmatter struct {
	ShortDescription string `yaml:"short_description" json:"short_description,omitempty"`
	LongDescription  string `yaml:"long_description" json:"long_description,omitempty"`
}

// ParseFrontmatter splits a markdown file into frontmatter and body.
// Returns empty frontmatter if none is found.
func ParseFrontmatter(content string) (Frontmatter, string) {
	var fm Frontmatter

	// Handle optional BOM
	trimmed := strings.TrimPrefix(content, "\xef\xbb\xbf")

	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return fm, content
	}

	// Find closing ---
	rest := trimmed[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx == -1 {
		// Try \r\n
		idx = strings.Index(rest, "\r\n---\r\n")
		if idx == -1 {
			return fm, content
		}
	}

	yamlContent := rest[:idx]
	body := rest[idx:]
	// Skip the closing "---\n" line
	if after, ok := strings.CutPrefix(body, "\n---\n"); ok {
		body = after
	} else if after, ok := strings.CutPrefix(body, "\r\n---\r\n"); ok {
		body = after
	}

	// Strip one optional leading blank line from body
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\r\n")

	_ = yaml.Unmarshal([]byte(yamlContent), &fm)
	return fm, body
}

// NormalizePath cleans up user-provided path input.
func NormalizePath(p string) (string, error) {
	// Convert backslashes to forward slashes
	p = strings.ReplaceAll(p, "\\", "/")
	// Clean
	p = path.Clean(p)
	// Reject ..
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("invalid path: must not contain ..")
	}
	// Strip leading /
	p = strings.TrimPrefix(p, "/")
	// Treat . as empty (root)
	if p == "." {
		p = ""
	}
	// Strip trailing .md
	p = strings.TrimSuffix(p, ".md")
	return p, nil
}

// ResolveResult describes what a path resolved to.
type ResolveResult struct {
	IsDir        bool
	ResolvedPath string // the actual path in the FS (with .md for files)
}

// ResolvePath tries to find what the user-provided path refers to.
func ResolvePath(contentFS fs.FS, p string) (*ResolveResult, error) {
	if p == "" {
		return &ResolveResult{IsDir: false, ResolvedPath: "INDEX.md"}, nil
	}

	// 1. Try {path}.md as a file
	filePath := p + ".md"
	if info, err := fs.Stat(contentFS, filePath); err == nil && !info.IsDir() {
		return &ResolveResult{IsDir: false, ResolvedPath: filePath}, nil
	}

	// 2. Try {path} as a directory
	if info, err := fs.Stat(contentFS, p); err == nil && info.IsDir() {
		return &ResolveResult{IsDir: true, ResolvedPath: p}, nil
	}

	// 3. Try {path} as an exact file match
	if info, err := fs.Stat(contentFS, p); err == nil && !info.IsDir() {
		return &ResolveResult{IsDir: false, ResolvedPath: p}, nil
	}

	// 4. Case-insensitive fallback
	if result := caseInsensitiveResolve(contentFS, p); result != nil {
		return result, nil
	}

	// 5. Not found — generate suggestions
	suggestions := FindSuggestions(contentFS, p)
	msg := fmt.Sprintf("path not found: %s", p)
	if len(suggestions) > 0 {
		msg += "\n\nDid you mean:\n"
		for _, s := range suggestions {
			msg += fmt.Sprintf("  %s\n", s)
		}
	}
	return nil, fmt.Errorf("%s", msg)
}

// caseInsensitiveResolve walks the FS trying case-insensitive matches.
func caseInsensitiveResolve(contentFS fs.FS, p string) *ResolveResult {
	lowerTarget := strings.ToLower(p)

	// Collect all paths
	var allPaths []string
	fs.WalkDir(contentFS, ".", func(walkPath string, d fs.DirEntry, err error) error {
		if err != nil || walkPath == "." {
			return nil
		}
		allPaths = append(allPaths, walkPath)
		return nil
	})

	for _, candidate := range allPaths {
		// Try without .md
		withoutMD := strings.TrimSuffix(candidate, ".md")
		if strings.ToLower(withoutMD) == lowerTarget {
			info, err := fs.Stat(contentFS, candidate)
			if err != nil {
				continue
			}
			return &ResolveResult{IsDir: info.IsDir(), ResolvedPath: candidate}
		}
	}
	return nil
}

// FindSuggestions returns up to 5 paths similar to the target.
func FindSuggestions(contentFS fs.FS, target string) []string {
	target = strings.ToLower(target)
	type scored struct {
		path  string
		score int
	}
	var candidates []scored

	fs.WalkDir(contentFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return nil
		}
		docID := strings.TrimSuffix(p, ".md")
		score := Levenshtein(strings.ToLower(docID), target)
		candidates = append(candidates, scored{path: docID, score: score})
		return nil
	})

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	var result []string
	for i, c := range candidates {
		if i >= 5 {
			break
		}
		result = append(result, c.path)
	}
	return result
}

// Levenshtein computes edit distance between two strings.
func Levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// ReadFile reads and outputs a single file from the content FS.
func ReadFile(contentFS fs.FS, resolvedPath, requestedPath string, jsonOutput bool) error {
	data, err := fs.ReadFile(contentFS, resolvedPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", resolvedPath, err)
	}

	fm, body := ParseFrontmatter(string(data))

	if jsonOutput {
		out := map[string]any{
			"path":          requestedPath,
			"resolved_path": resolvedPath,
			"type":          "file",
			"frontmatter":   fm,
			"content":       body,
			"size":          len(data),
		}
		return WriteJSON(out)
	}

	fmt.Print(body)
	return nil
}

// ListDir lists the contents of a directory in the content FS.
func ListDir(contentFS fs.FS, dirPath string, jsonOutput bool) error {
	entries, err := fs.ReadDir(contentFS, dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	type entry struct {
		Name             string `json:"name"`
		Path             string `json:"path"`
		Type             string `json:"type"`
		ShortDescription string `json:"short_description,omitempty"`
		Size             int    `json:"size,omitempty"`
	}

	var items []entry
	for _, e := range entries {
		name := e.Name()
		// Skip INDEX.md from directory listings
		if strings.EqualFold(name, "INDEX.md") {
			continue
		}

		entryPath := path.Join(dirPath, name)
		docID := strings.TrimSuffix(name, ".md")
		entryType := "file"

		if e.IsDir() {
			entryType = "directory"
			docID = name
		}

		var desc string
		var size int
		if !e.IsDir() {
			data, err := fs.ReadFile(contentFS, entryPath)
			if err == nil {
				size = len(data)
				fm, _ := ParseFrontmatter(string(data))
				desc = fm.ShortDescription
			}
		}

		items = append(items, entry{
			Name:             docID,
			Path:             strings.TrimSuffix(entryPath, ".md"),
			Type:             entryType,
			ShortDescription: desc,
			Size:             size,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	if jsonOutput {
		out := map[string]any{
			"path":    dirPath,
			"type":    "directory",
			"entries": items,
		}
		return WriteJSON(out)
	}

	fmt.Printf("%s/\n\n", dirPath)
	for _, item := range items {
		suffix := ""
		if item.Type == "directory" {
			suffix = "/"
		}
		if item.ShortDescription != "" {
			fmt.Printf("  %-25s %s\n", item.Name+suffix, item.ShortDescription)
		} else {
			fmt.Printf("  %s%s\n", item.Name, suffix)
		}
	}
	return nil
}

// ListAll lists all doc paths in the content FS.
func ListAll(contentFS fs.FS, jsonOutput bool) error {
	type entry struct {
		Path             string `json:"path"`
		ShortDescription string `json:"short_description,omitempty"`
	}

	var entries []entry

	fs.WalkDir(contentFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "." {
			return nil
		}
		docID := strings.TrimSuffix(p, ".md")

		var desc string
		data, readErr := fs.ReadFile(contentFS, p)
		if readErr == nil {
			fm, _ := ParseFrontmatter(string(data))
			desc = fm.ShortDescription
		}

		entries = append(entries, entry{Path: docID, ShortDescription: desc})
		return nil
	})

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	if jsonOutput {
		out := map[string]any{
			"type":    "listing",
			"entries": entries,
		}
		return WriteJSON(out)
	}

	for _, e := range entries {
		fmt.Println(e.Path)
	}
	return nil
}

// GrepMatch represents a single grep match with context.
type GrepMatch struct {
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Content       string   `json:"content"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

// GrepOptions holds parameters for a grep search.
type GrepOptions struct {
	ScopePath  string
	Pattern    string
	IsRegex    bool
	Before     int
	After      int
	ListOnly   bool
	JSONOutput bool
}

// Grep searches files in the content FS for a pattern.
func Grep(contentFS fs.FS, opts GrepOptions) error {
	// Build matcher
	var matcher func(string) bool
	if opts.IsRegex {
		re, err := regexp.Compile(opts.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
		matcher = re.MatchString
	} else {
		pattern := opts.Pattern
		matcher = func(line string) bool {
			return strings.Contains(line, pattern)
		}
	}

	// Determine which files to search
	var filesToSearch []string

	if opts.ScopePath != "" {
		// Try to resolve the scope path
		result, err := ResolvePath(contentFS, opts.ScopePath)
		if err != nil {
			return err
		}
		if result.IsDir {
			// Search all files in this directory
			fs.WalkDir(contentFS, result.ResolvedPath, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				filesToSearch = append(filesToSearch, p)
				return nil
			})
		} else {
			filesToSearch = []string{result.ResolvedPath}
		}
	} else {
		// Search all files
		fs.WalkDir(contentFS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || p == "." {
				return nil
			}
			filesToSearch = append(filesToSearch, p)
			return nil
		})
	}

	sort.Strings(filesToSearch)

	var allMatches []GrepMatch
	matchingFiles := map[string]bool{}

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

			matchingFiles[filePath] = true
			lineNum := i + 1

			// Collect context
			var before, after []string
			for b := max(0, i-opts.Before); b < i; b++ {
				before = append(before, lines[b])
			}
			for a := i + 1; a <= min(len(lines)-1, i+opts.After); a++ {
				after = append(after, lines[a])
			}

			allMatches = append(allMatches, GrepMatch{
				File:          filePath,
				Line:          lineNum,
				Content:       line,
				ContextBefore: before,
				ContextAfter:  after,
			})
		}
	}

	if len(allMatches) == 0 {
		return fmt.Errorf("no matches found for: %s", opts.Pattern)
	}

	// --list mode: just file paths
	if opts.ListOnly {
		if opts.JSONOutput {
			var files []string
			for f := range matchingFiles {
				files = append(files, f)
			}
			sort.Strings(files)
			return WriteJSON(map[string]any{
				"pattern":    opts.Pattern,
				"is_regex":   opts.IsRegex,
				"file_count": len(files),
				"files":      files,
			})
		}
		var files []string
		for f := range matchingFiles {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Println(f)
		}
		return nil
	}

	if opts.JSONOutput {
		return WriteJSON(map[string]any{
			"pattern":     opts.Pattern,
			"is_regex":    opts.IsRegex,
			"match_count": len(allMatches),
			"file_count":  len(matchingFiles),
			"matches":     allMatches,
		})
	}

	// Standard grep output
	lastFile := ""
	lastEndLine := -1

	for _, m := range allMatches {
		startLine := m.Line - len(m.ContextBefore)

		// Print separator between non-adjacent matches
		if lastFile != "" && (m.File != lastFile || startLine > lastEndLine+1) {
			fmt.Println("--")
		}

		// Print context before
		for i, line := range m.ContextBefore {
			lineNum := m.Line - len(m.ContextBefore) + i
			fmt.Printf("%s-%d-%s\n", m.File, lineNum, line)
		}

		// Print match line
		fmt.Printf("%s:%d:%s\n", m.File, m.Line, m.Content)

		// Print context after
		for i, line := range m.ContextAfter {
			lineNum := m.Line + i + 1
			fmt.Printf("%s-%d-%s\n", m.File, lineNum, line)
		}

		lastFile = m.File
		lastEndLine = m.Line + len(m.ContextAfter)
	}

	return nil
}

// WriteJSON marshals v as indented JSON and prints it.
func WriteJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
