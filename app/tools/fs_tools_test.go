//go:build windows || darwin

package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceGuardRejectsPathOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		t.Fatalf("NewWorkspaceGuard() error = %v", err)
	}

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, _, err := guard.ResolvePath(outsideFile); err == nil {
		t.Fatal("ResolvePath() succeeded for a path outside the workspace")
	}
}

func TestFileReadToolReadsRelativePath(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		t.Fatalf("NewWorkspaceGuard() error = %v", err)
	}

	tool := &FileReadTool{guard: guard}
	value, content, err := tool.Execute(context.Background(), map[string]any{
		"path":      "src/main.go",
		"max_lines": 2,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	result, ok := value.(FileReadResult)
	if !ok {
		t.Fatalf("Execute() result type = %T, want FileReadResult", value)
	}
	if result.Path != "src/main.go" {
		t.Fatalf("Path = %q, want %q", result.Path, "src/main.go")
	}
	if result.StartLine != 1 || result.EndLine != 2 {
		t.Fatalf("line range = %d-%d, want 1-2", result.StartLine, result.EndLine)
	}
	if !strings.Contains(content, "package main") {
		t.Fatalf("content = %q, want file contents", content)
	}
}

func TestFileReadToolRejectsBinaryFile(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "blob.bin")
	if err := os.WriteFile(filePath, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		t.Fatalf("NewWorkspaceGuard() error = %v", err)
	}

	tool := &FileReadTool{guard: guard}
	if _, _, err := tool.Execute(context.Background(), map[string]any{"path": "blob.bin"}); err == nil || !strings.Contains(err.Error(), "binary") {
		t.Fatalf("Execute() error = %v, want binary-file rejection", err)
	}
}

func TestFileSearchAndGrepStayInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nconst workspaceOnly = true\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("workspaceOnly marker\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		t.Fatalf("NewWorkspaceGuard() error = %v", err)
	}

	searchTool := &FileSearchTool{guard: guard}
	searchValue, _, err := searchTool.Execute(context.Background(), map[string]any{"pattern": "main"})
	if err != nil {
		t.Fatalf("FileSearchTool.Execute() error = %v", err)
	}
	searchResult := searchValue.(FileSearchResult)
	if len(searchResult.Matches) != 1 || searchResult.Matches[0].Path != "main.go" {
		t.Fatalf("search matches = %#v, want only main.go", searchResult.Matches)
	}

	grepTool := &FileGrepTool{guard: guard}
	grepValue, _, err := grepTool.Execute(context.Background(), map[string]any{"query": "workspaceOnly"})
	if err != nil {
		t.Fatalf("FileGrepTool.Execute() error = %v", err)
	}
	grepResult := grepValue.(FileGrepResult)
	if len(grepResult.Matches) != 2 {
		t.Fatalf("grep matches = %#v, want 2 workspace matches", grepResult.Matches)
	}
	for _, match := range grepResult.Matches {
		if !strings.HasSuffix(match.Path, ".go") && !strings.HasSuffix(match.Path, ".txt") {
			t.Fatalf("unexpected grep match path %q", match.Path)
		}
	}
}

func TestWorkspaceGuardSymlinkStaysWithinWorkspace(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "target.txt")
	if err := os.WriteFile(targetPath, []byte("inside"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		if errors.Is(err, os.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "privilege") {
			t.Skipf("symlink creation is not available: %v", err)
		}
		t.Fatalf("Symlink() error = %v", err)
	}

	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		t.Fatalf("NewWorkspaceGuard() error = %v", err)
	}

	if _, _, err := guard.ResolvePath("link.txt"); err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
}

func TestRegisterFileToolsModeGatesWriteTools(t *testing.T) {
	root := t.TempDir()

	readOnly := NewRegistry()
	if err := RegisterFileTools(readOnly, root, FileToolsModeReadOnly); err != nil {
		t.Fatalf("RegisterFileTools(read_only) error = %v", err)
	}
	if _, ok := readOnly.Get("fs_write"); ok {
		t.Fatal("fs_write should not be registered in read_only mode")
	}

	approve := NewRegistry()
	if err := RegisterFileTools(approve, root, FileToolsModeApprove); err != nil {
		t.Fatalf("RegisterFileTools(approve) error = %v", err)
	}
	for _, name := range []string{"fs_write", "fs_patch", "fs_mkdir", "fs_move"} {
		if _, ok := approve.Get(name); !ok {
			t.Fatalf("%s should be registered in approve mode", name)
		}
	}
}

func TestFileWriteAndPatchTools(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		t.Fatalf("NewWorkspaceGuard() error = %v", err)
	}

	writeTool := &FileWriteTool{guard: guard}
	value, _, err := writeTool.Execute(context.Background(), map[string]any{
		"path":    "notes.txt",
		"content": "line one\nline two\n",
	})
	if err != nil {
		t.Fatalf("FileWriteTool.Execute() error = %v", err)
	}
	writeResult := value.(FileWriteResult)
	if !writeResult.Overwritten || writeResult.Path != "notes.txt" {
		t.Fatalf("unexpected write result: %#v", writeResult)
	}

	updated, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(updated) != "line one\nline two\n" {
		t.Fatalf("updated file = %q", updated)
	}

	patchTool := &FilePatchTool{guard: guard}
	patchValue, _, err := patchTool.Execute(context.Background(), map[string]any{
		"path": "notes.txt",
		"edits": []any{
			map[string]any{
				"old_text": "line two",
				"new_text": "patched line",
			},
		},
	})
	if err != nil {
		t.Fatalf("FilePatchTool.Execute() error = %v", err)
	}
	patchResult := patchValue.(FilePatchResult)
	if patchResult.TotalReplacements != 1 {
		t.Fatalf("TotalReplacements = %d, want 1", patchResult.TotalReplacements)
	}

	patched, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(patched), "patched line") {
		t.Fatalf("patched file = %q, want replacement applied", patched)
	}
}

func TestFileMkdirAndMoveTools(t *testing.T) {
	root := t.TempDir()
	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		t.Fatalf("NewWorkspaceGuard() error = %v", err)
	}

	mkdirTool := &FileMkdirTool{guard: guard}
	if _, _, err := mkdirTool.Execute(context.Background(), map[string]any{
		"path": "nested/output",
	}); err != nil {
		t.Fatalf("FileMkdirTool.Execute() error = %v", err)
	}

	sourcePath := filepath.Join(root, "nested", "output", "file.txt")
	if err := os.WriteFile(sourcePath, []byte("move me"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	moveTool := &FileMoveTool{guard: guard}
	moveValue, _, err := moveTool.Execute(context.Background(), map[string]any{
		"source_path":      "nested/output/file.txt",
		"destination_path": "nested/file.txt",
	})
	if err != nil {
		t.Fatalf("FileMoveTool.Execute() error = %v", err)
	}
	moveResult := moveValue.(FileMoveResult)
	if moveResult.SourcePath != "nested/output/file.txt" || moveResult.DestinationPath != "nested/file.txt" {
		t.Fatalf("unexpected move result: %#v", moveResult)
	}

	if _, err := os.Stat(filepath.Join(root, "nested", "file.txt")); err != nil {
		t.Fatalf("destination file missing after move: %v", err)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source file still exists after move: %v", err)
	}
}
