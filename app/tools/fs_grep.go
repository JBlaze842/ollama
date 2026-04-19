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

var errFSGrepLimitReached = errors.New("filesystem grep result limit reached")

type FileGrepMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type FileGrepResult struct {
	Query         string          `json:"query"`
	Scope         string          `json:"scope"`
	CaseSensitive bool            `json:"case_sensitive,omitempty"`
	Matches       []FileGrepMatch `json:"matches"`
	FilesSearched int             `json:"files_searched"`
	FilesSkipped  int             `json:"files_skipped"`
	Truncated     bool            `json:"truncated,omitempty"`
}

type FileGrepTool struct {
	guard *WorkspaceGuard
}

func (t *FileGrepTool) Name() string {
	return "fs_grep"
}

func (t *FileGrepTool) Description() string {
	return "Search text file contents inside the configured workspace for a string."
}

func (t *FileGrepTool) Prompt() string {
	return ""
}

func (t *FileGrepTool) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Text to search for inside workspace files."
			},
			"path": {
				"type": "string",
				"description": "Optional file or directory path inside the workspace to limit the search scope."
			},
			"case_sensitive": {
				"type": "boolean",
				"description": "Whether the search should match case exactly."
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of matching lines to return (default: 25, max: 100)."
			}
		},
		"required": ["query"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (t *FileGrepTool) Execute(_ context.Context, args map[string]any) (any, string, error) {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return nil, "", fmt.Errorf("query must be a non-empty string")
	}

	scope, _ := args["path"].(string)
	scopePath, scopeRel, err := t.guard.ResolveScope(scope)
	if err != nil {
		return nil, "", err
	}

	caseSensitive := getOptionalBool(args, "case_sensitive")
	limit := clampResultLimit(getOptionalInt(args, "max_results"), defaultFSGrepResults, maxFSGrepResults)
	result := FileGrepResult{
		Query:         query,
		Scope:         scopeRel,
		CaseSensitive: caseSensitive,
		Matches:       make([]FileGrepMatch, 0, limit),
	}

	queryCompare := query
	if !caseSensitive {
		queryCompare = strings.ToLower(query)
	}

	searchFile := func(path string) error {
		data, readTruncated, err := t.guard.ReadTextFile(path, maxFSTextFileBytes)
		if err != nil {
			result.FilesSkipped++
			return nil
		}

		result.FilesSearched++
		if readTruncated {
			result.Truncated = true
		}

		relPath, err := filepath.Rel(t.guard.Root(), path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		lines := splitNormalizedLines(normalizeFileText(data))
		for i, line := range lines {
			lineCompare := line
			if !caseSensitive {
				lineCompare = strings.ToLower(line)
			}
			if !strings.Contains(lineCompare, queryCompare) {
				continue
			}

			if len(result.Matches) >= limit {
				result.Truncated = true
				return errFSGrepLimitReached
			}

			result.Matches = append(result.Matches, FileGrepMatch{
				Path:    relPath,
				Line:    i + 1,
				Content: truncateGrepLine(line),
			})
		}

		return nil
	}

	info, err := os.Stat(scopePath)
	if err != nil {
		return nil, "", fmt.Errorf("stat grep scope: %w", err)
	}
	if !info.IsDir() {
		if err := searchFile(scopePath); err != nil && !errors.Is(err, errFSGrepLimitReached) {
			return nil, "", err
		}
		return result, formatFileGrepResults(result), nil
	}

	err = filepath.WalkDir(scopePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			resolvedPath, _, err := t.guard.ResolvePath(path)
			if err != nil {
				return nil
			}
			path = resolvedPath
		}

		return searchFile(path)
	})
	if err != nil && !errors.Is(err, errFSGrepLimitReached) {
		return nil, "", fmt.Errorf("walk workspace: %w", err)
	}

	return result, formatFileGrepResults(result), nil
}

func formatFileGrepResults(result FileGrepResult) string {
	if len(result.Matches) == 0 {
		return fmt.Sprintf("No text matches for %q were found in %s.", result.Query, result.Scope)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d text matches for %q in %s:\n", len(result.Matches), result.Query, result.Scope)
	for i, match := range result.Matches {
		fmt.Fprintf(&b, "%d. %s:%d %s\n", i+1, match.Path, match.Line, match.Content)
	}
	if result.Truncated {
		b.WriteString("Results were truncated at the configured limit or file-size cap.")
	}

	return strings.TrimSpace(b.String())
}

func truncateGrepLine(line string) string {
	const limit = 240
	line = strings.TrimSpace(line)
	if len(line) <= limit {
		return line
	}
	return line[:limit]
}
