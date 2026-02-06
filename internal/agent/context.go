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

type Frontmatter struct {
	ShortDescription string `yaml:"short_description" json:"short_description,omitempty"`
	LongDescription  string `yaml:"long_description" json:"long_description,omitempty"`
}

func ParseFrontmatter(content string) (Frontmatter, string) {
	var fm Frontmatter

	trimmed := strings.TrimPrefix(content, "\xef\xbb\xbf")

	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return fm, content
	}

	rest := trimmed[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx == -1 {
		idx = strings.Index(rest, "\r\n---\r\n")
		if idx == -1 {
			return fm, content
		}
	}

	yamlContent := rest[:idx]
	body := rest[idx:]
	if after, ok := strings.CutPrefix(body, "\n---\n"); ok {
		body = after
	} else if after, ok := strings.CutPrefix(body, "\r\n---\r\n"); ok {
		body = after
	}

	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\r\n")

	_ = yaml.Unmarshal([]byte(yamlContent), &fm)
	return fm, body
}

func NormalizePath(p string) (string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("invalid path: must not contain parent directory references")
	}
	p = strings.TrimPrefix(p, "/")
	if p == "." {
		p = ""
	}
	p = strings.TrimSuffix(p, ".md")
	return p, nil
}

type ResolveResult struct {
	IsDir        bool
	ResolvedPath string // actual FS path (with .md for files)
}

func ResolvePath(contentFS fs.FS, p string) (*ResolveResult, error) {
	if p == "" {
		return &ResolveResult{IsDir: false, ResolvedPath: "INDEX.md"}, nil
	}

	filePath := p + ".md"
	if info, err := fs.Stat(contentFS, filePath); err == nil && !info.IsDir() {
		return &ResolveResult{IsDir: false, ResolvedPath: filePath}, nil
	}

	if info, err := fs.Stat(contentFS, p); err == nil && info.IsDir() {
		return &ResolveResult{IsDir: true, ResolvedPath: p}, nil
	}

	if info, err := fs.Stat(contentFS, p); err == nil && !info.IsDir() {
		return &ResolveResult{IsDir: false, ResolvedPath: p}, nil
	}

	result, err := caseInsensitiveResolve(contentFS, p)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}

	suggestions, err := findSuggestions(contentFS, p)
	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("path not found: %s", p)
	if len(suggestions) > 0 {
		msg += "\n\nDid you mean:\n"
		for _, s := range suggestions {
			msg += fmt.Sprintf("  %s\n", s)
		}
	}
	return nil, fmt.Errorf("%s", msg)
}

func caseInsensitiveResolve(contentFS fs.FS, p string) (*ResolveResult, error) {
	lowerTarget := strings.ToLower(p)
	var result *ResolveResult
	err := fs.WalkDir(contentFS, ".", func(walkPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if walkPath == "." {
			return nil
		}
		withoutMD := strings.TrimSuffix(walkPath, ".md")
		if strings.ToLower(withoutMD) == lowerTarget {
			info, statErr := fs.Stat(contentFS, walkPath)
			if statErr == nil {
				result = &ResolveResult{IsDir: info.IsDir(), ResolvedPath: walkPath}
				return fs.SkipAll
			}
		}
		return nil
	})
	return result, err
}

func findSuggestions(contentFS fs.FS, target string) ([]string, error) {
	target = strings.ToLower(target)
	type scored struct {
		path  string
		score int
	}
	var candidates []scored

	err := fs.WalkDir(contentFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		docID := strings.TrimSuffix(p, ".md")
		score := levenshtein(strings.ToLower(docID), target)
		candidates = append(candidates, scored{path: docID, score: score})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	limit := min(len(candidates), 5)
	result := make([]string, limit)
	for i := range limit {
		result[i] = candidates[i].path
	}
	return result, nil
}

func levenshtein(a, b string) int {
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
		return writeJSON(out)
	}

	fmt.Print(body)
	return nil
}

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
		return writeJSON(out)
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

func ListAll(contentFS fs.FS, jsonOutput bool) error {
	type entry struct {
		Path             string `json:"path"`
		ShortDescription string `json:"short_description,omitempty"`
	}

	var entries []entry

	if err := fs.WalkDir(contentFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == "." {
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
	}); err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	if jsonOutput {
		out := map[string]any{
			"type":    "listing",
			"entries": entries,
		}
		return writeJSON(out)
	}

	for _, e := range entries {
		fmt.Println(e.Path)
	}
	return nil
}

type GrepMatch struct {
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Content       string   `json:"content"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

type GrepOptions struct {
	ScopePath  string
	Pattern    string
	IsRegex    bool
	Before     int
	After      int
	ListOnly   bool
	JSONOutput bool
}

func Grep(contentFS fs.FS, opts GrepOptions) error {
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

	var filesToSearch []string

	if opts.ScopePath != "" {
		result, err := ResolvePath(contentFS, opts.ScopePath)
		if err != nil {
			return err
		}
		if result.IsDir {
			if err := fs.WalkDir(contentFS, result.ResolvedPath, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				filesToSearch = append(filesToSearch, p)
				return nil
			}); err != nil {
				return err
			}
		} else {
			filesToSearch = []string{result.ResolvedPath}
		}
	} else {
		if err := fs.WalkDir(contentFS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || p == "." {
				return nil
			}
			filesToSearch = append(filesToSearch, p)
			return nil
		}); err != nil {
			return err
		}
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
		return fmt.Errorf(`no matches found for: %s

This may indicate missing guidance in the agent context documentation.
Please submit feedback so we can improve the docs:

  speakeasy agent feedback --type missing_guidance --message "Searched for '%s' but found no results. Context: <describe what you were trying to accomplish>"

Your feedback helps us identify gaps in the documentation.`, opts.Pattern, opts.Pattern)
	}

	if opts.ListOnly {
		files := make([]string, 0, len(matchingFiles))
		for f := range matchingFiles {
			files = append(files, f)
		}
		sort.Strings(files)

		if opts.JSONOutput {
			return writeJSON(map[string]any{
				"pattern":    opts.Pattern,
				"is_regex":   opts.IsRegex,
				"file_count": len(files),
				"files":      files,
			})
		}
		for _, f := range files {
			fmt.Println(f)
		}
		return nil
	}

	if opts.JSONOutput {
		return writeJSON(map[string]any{
			"pattern":     opts.Pattern,
			"is_regex":    opts.IsRegex,
			"match_count": len(allMatches),
			"file_count":  len(matchingFiles),
			"matches":     allMatches,
		})
	}

	lastFile := ""
	lastEndLine := -1

	for _, m := range allMatches {
		startLine := m.Line - len(m.ContextBefore)

		// Separator between non-adjacent matches
		if lastFile != "" && (m.File != lastFile || startLine > lastEndLine+1) {
			fmt.Println("--")
		}

		for i, line := range m.ContextBefore {
			lineNum := m.Line - len(m.ContextBefore) + i
			fmt.Printf("%s-%d-%s\n", m.File, lineNum, line)
		}

		fmt.Printf("%s:%d:%s\n", m.File, m.Line, m.Content)

		for i, line := range m.ContextAfter {
			lineNum := m.Line + i + 1
			fmt.Printf("%s-%d-%s\n", m.File, lineNum, line)
		}

		lastFile = m.File
		lastEndLine = m.Line + len(m.ContextAfter)
	}

	return nil
}

func writeJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
