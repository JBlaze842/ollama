//go:build windows || darwin

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileWriteResult struct {
	Path        string `json:"path"`
	Created     bool   `json:"created,omitempty"`
	Overwritten bool   `json:"overwritten,omitempty"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	TotalLines  int    `json:"total_lines"`
	Truncated   bool   `json:"truncated,omitempty"`
	Content     string `json:"content"`
	Bytes       int    `json:"bytes"`
}

type FileWriteTool struct {
	guard *WorkspaceGuard
}

func (t *FileWriteTool) Name() string {
	return "fs_write"
}

func (t *FileWriteTool) Description() string {
	return "Create or overwrite a text file inside the configured workspace."
}

func (t *FileWriteTool) Prompt() string {
	return ""
}

func (t *FileWriteTool) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file inside the workspace."
			},
			"content": {
				"type": "string",
				"description": "Full text content to write to the file."
			}
		},
		"required": ["path", "content"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (t *FileWriteTool) RequiresApproval() bool {
	return true
}

func (t *FileWriteTool) Preview(args map[string]any) (any, string, error) {
	return t.prepare(args)
}

func (t *FileWriteTool) Execute(_ context.Context, args map[string]any) (any, string, error) {
	result, content, path, payload, perm, err := t.prepareWrite(args)
	if err != nil {
		return nil, "", err
	}

	if err := writeTextFileAtomic(path, payload, perm); err != nil {
		return nil, "", err
	}

	return result, content, nil
}

func (t *FileWriteTool) prepare(args map[string]any) (any, string, error) {
	result, content, _, _, _, err := t.prepareWrite(args)
	if err != nil {
		return nil, "", err
	}
	return result, content, nil
}

func (t *FileWriteTool) prepareWrite(args map[string]any) (FileWriteResult, string, string, []byte, os.FileMode, error) {
	path, ok := args["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("path must be a non-empty string")
	}

	content, ok := args["content"].(string)
	if !ok {
		return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("content must be a string")
	}

	resolvedPath, relPath, err := t.guard.ResolveTargetPath(path)
	if err != nil {
		return FileWriteResult{}, "", "", nil, 0, err
	}

	parentPath := filepath.Dir(resolvedPath)
	parentInfo, err := os.Stat(parentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("parent directory %q does not exist", filepath.ToSlash(filepath.Dir(relPath)))
		}
		return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("stat parent directory: %w", err)
	}
	if !parentInfo.IsDir() {
		return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("parent path %q is not a directory", filepath.ToSlash(filepath.Dir(relPath)))
	}

	payload := []byte(content)
	if len(payload) > maxFSTextFileBytes {
		return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("content exceeds the maximum size of %d bytes", maxFSTextFileBytes)
	}

	created := true
	overwritten := false
	perm := os.FileMode(0o644)
	if info, err := os.Stat(resolvedPath); err == nil {
		if info.IsDir() {
			return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("path %q is a directory", relPath)
		}
		created = false
		overwritten = true
		perm = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return FileWriteResult{}, "", "", nil, 0, fmt.Errorf("stat file: %w", err)
	}

	result := previewTextWrite(relPath, content, created, overwritten, len(payload))
	return result, formatFileWriteResult(result), resolvedPath, payload, perm, nil
}

func previewTextWrite(path string, content string, created bool, overwritten bool, size int) FileWriteResult {
	normalized := normalizeFileText([]byte(content))
	lines := splitNormalizedLines(normalized)

	totalLines := len(lines)
	if totalLines == 0 && normalized == "" {
		totalLines = 0
	}

	previewLines := lines
	truncated := false
	if len(previewLines) > defaultFSPreviewLines {
		previewLines = previewLines[:defaultFSPreviewLines]
		truncated = true
	}

	startLine := 1
	endLine := len(previewLines)
	preview := strings.Join(previewLines, "\n")
	if totalLines == 0 {
		startLine = 1
		endLine = 0
		preview = ""
	}

	return FileWriteResult{
		Path:        path,
		Created:     created,
		Overwritten: overwritten,
		StartLine:   startLine,
		EndLine:     endLine,
		TotalLines:  totalLines,
		Truncated:   truncated,
		Content:     preview,
		Bytes:       size,
	}
}

func formatFileWriteResult(result FileWriteResult) string {
	action := "Wrote"
	switch {
	case result.Created:
		action = "Created"
	case result.Overwritten:
		action = "Updated"
	}

	if result.TotalLines == 0 {
		return fmt.Sprintf("%s %s with empty content.", action, result.Path)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s lines %d-%d of %d", action, result.Path, result.StartLine, result.EndLine, result.TotalLines)
	if result.Truncated {
		b.WriteString(" (preview truncated)")
	}
	b.WriteString(":\n")
	b.WriteString(result.Content)

	return strings.TrimSpace(b.String())
}

func writeTextFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".ollama-fs-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tempPath := tempFile.Name()
	success := false
	defer func() {
		_ = tempFile.Close()
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if perm != 0 {
		if err := os.Chmod(tempPath, perm); err != nil {
			return fmt.Errorf("chmod temp file: %w", err)
		}
	}

	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove existing file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat destination file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	success = true
	return nil
}
