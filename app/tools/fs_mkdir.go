//go:build windows || darwin

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FileMkdirResult struct {
	Path          string `json:"path"`
	Created       bool   `json:"created,omitempty"`
	AlreadyExists bool   `json:"already_exists,omitempty"`
}

type FileMkdirTool struct {
	guard *WorkspaceGuard
}

func (t *FileMkdirTool) Name() string {
	return "fs_mkdir"
}

func (t *FileMkdirTool) Description() string {
	return "Create a directory inside the configured workspace."
}

func (t *FileMkdirTool) Prompt() string {
	return ""
}

func (t *FileMkdirTool) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Directory path to create inside the workspace."
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

func (t *FileMkdirTool) RequiresApproval() bool {
	return true
}

func (t *FileMkdirTool) Preview(args map[string]any) (any, string, error) {
	result, _, content, err := t.prepare(args)
	if err != nil {
		return nil, "", err
	}
	return result, content, nil
}

func (t *FileMkdirTool) Execute(_ context.Context, args map[string]any) (any, string, error) {
	result, path, content, err := t.prepare(args)
	if err != nil {
		return nil, "", err
	}

	if !result.AlreadyExists {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, "", fmt.Errorf("create directory: %w", err)
		}
	}

	return result, content, nil
}

func (t *FileMkdirTool) prepare(args map[string]any) (FileMkdirResult, string, string, error) {
	path, ok := args["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return FileMkdirResult{}, "", "", fmt.Errorf("path must be a non-empty string")
	}

	resolvedPath, relPath, err := t.guard.ResolveTargetPath(path)
	if err != nil {
		return FileMkdirResult{}, "", "", err
	}

	result := FileMkdirResult{
		Path:    relPath,
		Created: true,
	}

	if info, err := os.Stat(resolvedPath); err == nil {
		if !info.IsDir() {
			return FileMkdirResult{}, "", "", fmt.Errorf("path %q already exists as a file", relPath)
		}
		result.Created = false
		result.AlreadyExists = true
	} else if !os.IsNotExist(err) {
		return FileMkdirResult{}, "", "", fmt.Errorf("stat directory: %w", err)
	}

	return result, resolvedPath, formatFileMkdirResult(result), nil
}

func formatFileMkdirResult(result FileMkdirResult) string {
	if result.AlreadyExists {
		return fmt.Sprintf("Directory %s already exists.", result.Path)
	}
	return fmt.Sprintf("Create directory %s.", result.Path)
}
