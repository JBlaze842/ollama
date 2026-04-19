//go:build windows || darwin

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var errFSSearchLimitReached = errors.New("filesystem search result limit reached")

type FileSearchMatch struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type FileSearchResult struct {
	Pattern   string            `json:"pattern"`
	Scope     string            `json:"scope"`
	Matches   []FileSearchMatch `json:"matches"`
	Truncated bool              `json:"truncated,omitempty"`
}

type FileSearchTool struct {
	guard *WorkspaceGuard
}

func (t *FileSearchTool) Name() string {
	return "fs_search"
}

func (t *FileSearchTool) Description() string {
	return "Search for files and directories inside the configured workspace by matching a path or filename pattern."
}

func (t *FileSearchTool) Prompt() string {
	return ""
}

func (t *FileSearchTool) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Case-insensitive filename or path fragment to search for inside the workspace."
			},
			"path": {
				"type": "string",
				"description": "Optional file or directory path inside the workspace to limit the search scope."
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of matches to return (default: 25, max: 100)."
			}
		},
		"required": ["pattern"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (t *FileSearchTool) Execute(_ context.Context, args map[string]any) (any, string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || strings.TrimSpace(pattern) == "" {
		return nil, "", fmt.Errorf("pattern must be a non-empty string")
	}

	scope, _ := args["path"].(string)
	scopePath, scopeRel, err := t.guard.ResolveScope(scope)
	if err != nil {
		return nil, "", err
	}

	limit := clampResultLimit(getOptionalInt(args, "max_results"), defaultFSSearchResults, maxFSSearchResults)
	result := FileSearchResult{
		Pattern: pattern,
		Scope:   scopeRel,
		Matches: make([]FileSearchMatch, 0, limit),
	}

	matchNeedle := strings.ToLower(strings.TrimSpace(pattern))
	appendMatch := func(path string, entryType string) error {
		relPath, err := filepath.Rel(t.guard.Root(), path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		relPath = filepath.ToSlash(relPath)
		name := strings.ToLower(filepath.Base(relPath))
		if !strings.Contains(strings.ToLower(relPath), matchNeedle) && !strings.Contains(name, matchNeedle) {
			return nil
		}

		if len(result.Matches) >= limit {
			result.Truncated = true
			return errFSSearchLimitReached
		}

		result.Matches = append(result.Matches, FileSearchMatch{
			Path: relPath,
			Type: entryType,
		})
		return nil
	}

	info, err := os.Lstat(scopePath)
	if err != nil {
		return nil, "", fmt.Errorf("stat search scope: %w", err)
	}

	if !info.IsDir() {
		if err := appendMatch(scopePath, "file"); err != nil && !errors.Is(err, errFSSearchLimitReached) {
			return nil, "", err
		}
		return result, formatFileSearchResults(result), nil
	}

	err = filepath.WalkDir(scopePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		entryPath := path
		entryType := "file"
		if d.IsDir() {
			entryType = "dir"
		}
		if d.Type()&os.ModeSymlink != 0 {
			resolvedPath, _, err := t.guard.ResolvePath(path)
			if err != nil {
				return nil
			}
			entryPath = resolvedPath
			entryType = "symlink"
		}

		return appendMatch(entryPath, entryType)
	})
	if err != nil && !errors.Is(err, errFSSearchLimitReached) {
		return nil, "", fmt.Errorf("walk workspace: %w", err)
	}

	return result, formatFileSearchResults(result), nil
}

func formatFileSearchResults(result FileSearchResult) string {
	if len(result.Matches) == 0 {
		return fmt.Sprintf("No files or directories matched %q in %s.", result.Pattern, result.Scope)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d workspace matches for %q in %s:\n", len(result.Matches), result.Pattern, result.Scope)
	for i, match := range result.Matches {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, match.Type, match.Path)
	}
	if result.Truncated {
		b.WriteString("Results were truncated at the configured limit.")
	}

	return strings.TrimSpace(b.String())
}
