//go:build windows || darwin

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FilePatchEdit struct {
	OldText    string `json:"old_text"`
	NewText    string `json:"new_text"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
	Matches    int    `json:"matches"`
}

type FilePatchResult struct {
	Path              string          `json:"path"`
	Edits             []FilePatchEdit `json:"edits"`
	TotalReplacements int             `json:"total_replacements"`
}

type FilePatchTool struct {
	guard *WorkspaceGuard
}

func (t *FilePatchTool) Name() string {
	return "fs_patch"
}

func (t *FilePatchTool) Description() string {
	return "Apply exact text replacements to a text file inside the configured workspace."
}

func (t *FilePatchTool) Prompt() string {
	return ""
}

func (t *FilePatchTool) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file inside the workspace."
			},
			"edits": {
				"type": "array",
				"description": "Ordered exact-match replacements to apply to the file.",
				"items": {
					"type": "object",
					"properties": {
						"old_text": {
							"type": "string",
							"description": "Exact text to replace."
						},
						"new_text": {
							"type": "string",
							"description": "Replacement text."
						},
						"replace_all": {
							"type": "boolean",
							"description": "Whether to replace every remaining match instead of exactly one."
						}
					},
					"required": ["old_text", "new_text"]
				}
			}
		},
		"required": ["path", "edits"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (t *FilePatchTool) RequiresApproval() bool {
	return true
}

func (t *FilePatchTool) Preview(args map[string]any) (any, string, error) {
	result, _, content, _, _, err := t.prepare(args)
	if err != nil {
		return nil, "", err
	}
	return result, content, nil
}

func (t *FilePatchTool) Execute(_ context.Context, args map[string]any) (any, string, error) {
	result, path, content, nextContent, perm, err := t.prepare(args)
	if err != nil {
		return nil, "", err
	}

	if err := writeTextFileAtomic(path, []byte(nextContent), perm); err != nil {
		return nil, "", err
	}

	return result, content, nil
}

func (t *FilePatchTool) prepare(args map[string]any) (FilePatchResult, string, string, string, os.FileMode, error) {
	path, ok := args["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return FilePatchResult{}, "", "", "", 0, fmt.Errorf("path must be a non-empty string")
	}

	edits, err := parseFilePatchEdits(args["edits"])
	if err != nil {
		return FilePatchResult{}, "", "", "", 0, err
	}

	resolvedPath, relPath, err := t.guard.ResolvePath(path)
	if err != nil {
		return FilePatchResult{}, "", "", "", 0, err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return FilePatchResult{}, "", "", "", 0, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return FilePatchResult{}, "", "", "", 0, fmt.Errorf("path %q is a directory", relPath)
	}

	data, _, err := t.guard.ReadTextFile(resolvedPath, maxFSTextFileBytes)
	if err != nil {
		return FilePatchResult{}, "", "", "", 0, err
	}

	current := string(data)
	working := current
	result := FilePatchResult{
		Path:  relPath,
		Edits: make([]FilePatchEdit, 0, len(edits)),
	}

	for _, edit := range edits {
		matchCount := strings.Count(working, edit.OldText)
		if matchCount == 0 {
			return FilePatchResult{}, "", "", "", 0, fmt.Errorf("old_text was not found in %s", relPath)
		}
		if !edit.ReplaceAll && matchCount != 1 {
			return FilePatchResult{}, "", "", "", 0, fmt.Errorf("old_text matched %d locations in %s; set replace_all to true or provide a more specific edit", matchCount, relPath)
		}

		replacements := 1
		if edit.ReplaceAll {
			replacements = matchCount
			working = strings.ReplaceAll(working, edit.OldText, edit.NewText)
		} else {
			working = strings.Replace(working, edit.OldText, edit.NewText, 1)
		}

		result.Edits = append(result.Edits, FilePatchEdit{
			OldText:    edit.OldText,
			NewText:    edit.NewText,
			ReplaceAll: edit.ReplaceAll,
			Matches:    replacements,
		})
		result.TotalReplacements += replacements
	}

	return result, resolvedPath, formatFilePatchResult(result), working, info.Mode().Perm(), nil
}

func formatFilePatchResult(result FilePatchResult) string {
	if len(result.Edits) == 0 {
		return fmt.Sprintf("No edits were applied to %s.", result.Path)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Prepared %d edit(s) for %s (%d replacement(s)).\n", len(result.Edits), result.Path, result.TotalReplacements)
	for i, edit := range result.Edits {
		fmt.Fprintf(&b, "%d. %d replacement(s)%s\n", i+1, edit.Matches, ternaryString(edit.ReplaceAll, " (replace_all)", ""))
	}

	return strings.TrimSpace(b.String())
}

type filePatchSpec struct {
	OldText    string
	NewText    string
	ReplaceAll bool
}

func parseFilePatchEdits(value any) ([]filePatchSpec, error) {
	rawEdits, ok := value.([]any)
	if !ok || len(rawEdits) == 0 {
		return nil, fmt.Errorf("edits must be a non-empty array")
	}

	edits := make([]filePatchSpec, 0, len(rawEdits))
	for _, rawEdit := range rawEdits {
		editMap, ok := rawEdit.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each edit must be an object")
		}

		oldText, ok := editMap["old_text"].(string)
		if !ok || oldText == "" {
			return nil, fmt.Errorf("each edit.old_text must be a non-empty string")
		}

		newText, ok := editMap["new_text"].(string)
		if !ok {
			return nil, fmt.Errorf("each edit.new_text must be a string")
		}

		edits = append(edits, filePatchSpec{
			OldText:    oldText,
			NewText:    newText,
			ReplaceAll: getOptionalBool(editMap, "replace_all"),
		})
	}

	return edits, nil
}

func ternaryString(condition bool, whenTrue string, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}
