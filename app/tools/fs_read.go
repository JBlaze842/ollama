//go:build windows || darwin

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FileReadResult struct {
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	TotalLines int    `json:"total_lines"`
	Truncated  bool   `json:"truncated,omitempty"`
	Content    string `json:"content"`
}

type FileReadTool struct {
	guard *WorkspaceGuard
}

func (t *FileReadTool) Name() string {
	return "fs_read"
}

func (t *FileReadTool) Description() string {
	return "Read a text file inside the configured workspace with line limits."
}

func (t *FileReadTool) Prompt() string {
	return ""
}

func (t *FileReadTool) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file inside the workspace."
			},
			"start_line": {
				"type": "integer",
				"description": "1-based starting line number (default: 1)."
			},
			"max_lines": {
				"type": "integer",
				"description": "Maximum number of lines to return (default: 200, max: 500)."
			}
		},
		"required": ["path"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (t *FileReadTool) Execute(_ context.Context, args map[string]any) (any, string, error) {
	path, ok := args["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return nil, "", fmt.Errorf("path must be a non-empty string")
	}

	resolvedPath, relPath, err := t.guard.ResolvePath(path)
	if err != nil {
		return nil, "", err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("path %q is a directory", relPath)
	}

	startLine := getOptionalInt(args, "start_line")
	if startLine <= 0 {
		startLine = 1
	}
	maxLines := clampLineCount(getOptionalInt(args, "max_lines"))

	data, readTruncated, err := t.guard.ReadTextFile(resolvedPath, maxFSTextFileBytes)
	if err != nil {
		return nil, "", err
	}

	lines := splitNormalizedLines(normalizeFileText(data))
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{}
	}

	if len(lines) == 0 {
		result := FileReadResult{
			Path:       relPath,
			StartLine:  1,
			EndLine:    0,
			TotalLines: 0,
			Truncated:  readTruncated,
			Content:    "",
		}
		return result, formatFileReadResult(result), nil
	}

	if startLine > len(lines) {
		return nil, "", fmt.Errorf("start_line %d is beyond the end of %s (%d total lines)", startLine, relPath, len(lines))
	}

	endLine := startLine + maxLines - 1
	if endLine > len(lines) {
		endLine = len(lines)
	}

	result := FileReadResult{
		Path:       relPath,
		StartLine:  startLine,
		EndLine:    endLine,
		TotalLines: len(lines),
		Truncated:  readTruncated || endLine < len(lines),
		Content:    strings.Join(lines[startLine-1:endLine], "\n"),
	}

	return result, formatFileReadResult(result), nil
}

func formatFileReadResult(result FileReadResult) string {
	if result.TotalLines == 0 {
		return fmt.Sprintf("Read %s: file is empty.", result.Path)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Read %s lines %d-%d of %d", result.Path, result.StartLine, result.EndLine, result.TotalLines)
	if result.Truncated {
		b.WriteString(" (truncated)")
	}
	b.WriteString(":\n")
	b.WriteString(result.Content)

	return strings.TrimSpace(b.String())
}
