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

type FileMoveResult struct {
	SourcePath       string `json:"source_path"`
	DestinationPath  string `json:"destination_path"`
	SourceType       string `json:"source_type"`
	Overwritten      bool   `json:"overwritten,omitempty"`
	DestinationExist bool   `json:"destination_exists,omitempty"`
}

type FileMoveTool struct {
	guard *WorkspaceGuard
}

func (t *FileMoveTool) Name() string {
	return "fs_move"
}

func (t *FileMoveTool) Description() string {
	return "Move or rename a file or directory inside the configured workspace."
}

func (t *FileMoveTool) Prompt() string {
	return ""
}

func (t *FileMoveTool) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"source_path": {
				"type": "string",
				"description": "Existing file or directory path inside the workspace."
			},
			"destination_path": {
				"type": "string",
				"description": "New path inside the workspace."
			},
			"overwrite": {
				"type": "boolean",
				"description": "Whether an existing destination should be replaced."
			}
		},
		"required": ["source_path", "destination_path"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (t *FileMoveTool) RequiresApproval() bool {
	return true
}

func (t *FileMoveTool) Preview(args map[string]any) (any, string, error) {
	result, _, _, content, err := t.prepare(args)
	if err != nil {
		return nil, "", err
	}
	return result, content, nil
}

func (t *FileMoveTool) Execute(_ context.Context, args map[string]any) (any, string, error) {
	result, sourcePath, destinationPath, content, err := t.prepare(args)
	if err != nil {
		return nil, "", err
	}

	if result.DestinationExist {
		if err := os.RemoveAll(destinationPath); err != nil {
			return nil, "", fmt.Errorf("remove existing destination: %w", err)
		}
	}

	if err := os.Rename(sourcePath, destinationPath); err != nil {
		return nil, "", fmt.Errorf("move path: %w", err)
	}

	return result, content, nil
}

func (t *FileMoveTool) prepare(args map[string]any) (FileMoveResult, string, string, string, error) {
	source, ok := args["source_path"].(string)
	if !ok || strings.TrimSpace(source) == "" {
		return FileMoveResult{}, "", "", "", fmt.Errorf("source_path must be a non-empty string")
	}

	destination, ok := args["destination_path"].(string)
	if !ok || strings.TrimSpace(destination) == "" {
		return FileMoveResult{}, "", "", "", fmt.Errorf("destination_path must be a non-empty string")
	}

	sourcePath, sourceRel, err := t.guard.ResolvePath(source)
	if err != nil {
		return FileMoveResult{}, "", "", "", err
	}
	destinationPath, destinationRel, err := t.guard.ResolveTargetPath(destination)
	if err != nil {
		return FileMoveResult{}, "", "", "", err
	}

	if sourcePath == destinationPath {
		return FileMoveResult{}, "", "", "", fmt.Errorf("source_path and destination_path must be different")
	}

	parentPath := filepath.Dir(destinationPath)
	parentInfo, err := os.Stat(parentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileMoveResult{}, "", "", "", fmt.Errorf("destination parent %q does not exist", filepath.ToSlash(filepath.Dir(destinationRel)))
		}
		return FileMoveResult{}, "", "", "", fmt.Errorf("stat destination parent: %w", err)
	}
	if !parentInfo.IsDir() {
		return FileMoveResult{}, "", "", "", fmt.Errorf("destination parent %q is not a directory", filepath.ToSlash(filepath.Dir(destinationRel)))
	}

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return FileMoveResult{}, "", "", "", fmt.Errorf("stat source path: %w", err)
	}

	overwrite := getOptionalBool(args, "overwrite")
	result := FileMoveResult{
		SourcePath:      sourceRel,
		DestinationPath: destinationRel,
		SourceType:      ternaryString(sourceInfo.IsDir(), "dir", "file"),
	}

	if _, err := os.Stat(destinationPath); err == nil {
		result.DestinationExist = true
		if !overwrite {
			return FileMoveResult{}, "", "", "", fmt.Errorf("destination_path %q already exists", destinationRel)
		}
		result.Overwritten = true
	} else if !os.IsNotExist(err) {
		return FileMoveResult{}, "", "", "", fmt.Errorf("stat destination path: %w", err)
	}

	return result, sourcePath, destinationPath, formatFileMoveResult(result), nil
}

func formatFileMoveResult(result FileMoveResult) string {
	action := "Move"
	if result.SourceType == "dir" {
		action = "Move directory"
	}
	if result.SourceType == "file" {
		action = "Move file"
	}
	summary := fmt.Sprintf("%s %s to %s.", action, result.SourcePath, result.DestinationPath)
	if result.Overwritten {
		summary += " Existing destination will be replaced."
	}
	return summary
}
