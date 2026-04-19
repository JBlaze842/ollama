//go:build windows || darwin

package tools

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	FileToolsModeOff      = "off"
	FileToolsModeReadOnly = "read_only"
	FileToolsModeApprove  = "approve"
	FileToolsModeFullAuto = "full_auto"

	defaultFSReadLines     = 200
	maxFSReadLines         = 500
	defaultFSSearchResults = 25
	maxFSSearchResults     = 100
	defaultFSGrepResults   = 25
	maxFSGrepResults       = 100
	defaultFSPreviewLines  = 120
	maxFSTextFileBytes     = 512 * 1024
)

type WorkspaceGuard struct {
	root        string
	compareRoot string
}

func NormalizeFileToolsMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", FileToolsModeOff:
		return FileToolsModeOff
	case FileToolsModeReadOnly:
		return FileToolsModeReadOnly
	case FileToolsModeApprove:
		return FileToolsModeApprove
	case FileToolsModeFullAuto:
		return FileToolsModeFullAuto
	default:
		return FileToolsModeOff
	}
}

func NewWorkspaceGuard(root string) (*WorkspaceGuard, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("workspace root is not configured")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root symlinks: %w", err)
	}

	info, err := os.Stat(resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("stat workspace root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace root must be a directory")
	}

	return &WorkspaceGuard{
		root:        resolvedRoot,
		compareRoot: normalizeComparisonPath(resolvedRoot),
	}, nil
}

func RegisterReadOnlyFileTools(registry *Registry, root string) error {
	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		return err
	}

	registry.Register(&FileSearchTool{guard: guard})
	registry.Register(&FileGrepTool{guard: guard})
	registry.Register(&FileReadTool{guard: guard})

	return nil
}

func RegisterFileTools(registry *Registry, root string, mode string) error {
	guard, err := NewWorkspaceGuard(root)
	if err != nil {
		return err
	}

	registry.Register(&FileSearchTool{guard: guard})
	registry.Register(&FileGrepTool{guard: guard})
	registry.Register(&FileReadTool{guard: guard})

	switch NormalizeFileToolsMode(mode) {
	case FileToolsModeApprove, FileToolsModeFullAuto:
		registry.Register(&FileWriteTool{guard: guard})
		registry.Register(&FilePatchTool{guard: guard})
		registry.Register(&FileMkdirTool{guard: guard})
		registry.Register(&FileMoveTool{guard: guard})
	}

	return nil
}

func (g *WorkspaceGuard) Root() string {
	return g.root
}

func (g *WorkspaceGuard) ResolvePath(path string) (string, string, error) {
	return g.resolvePath(path)
}

func (g *WorkspaceGuard) ResolveTargetPath(path string) (string, string, error) {
	if strings.TrimSpace(path) == "" {
		return "", "", fmt.Errorf("path must be provided")
	}

	candidate := filepath.Clean(path)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(g.root, candidate)
	}

	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}

	if _, err := os.Lstat(absPath); err == nil {
		return g.resolvePath(absPath)
	} else if !os.IsNotExist(err) {
		return "", "", fmt.Errorf("stat path: %w", err)
	}

	existingParent := filepath.Dir(absPath)
	missingParts := []string{filepath.Base(absPath)}

	for {
		if existingParent == "" || existingParent == filepath.Dir(existingParent) && !filepath.IsAbs(existingParent) {
			return "", "", fmt.Errorf("path %q is outside the configured workspace", path)
		}

		if _, err := os.Lstat(existingParent); err == nil {
			resolvedParent, _, err := g.resolvePath(existingParent)
			if err != nil {
				return "", "", err
			}

			resolvedPath := filepath.Join(append([]string{resolvedParent}, missingParts...)...)
			if !g.contains(resolvedPath) {
				return "", "", fmt.Errorf("path %q is outside the configured workspace", path)
			}

			relPath, err := filepath.Rel(g.root, resolvedPath)
			if err != nil {
				return "", "", fmt.Errorf("resolve relative path: %w", err)
			}
			return resolvedPath, filepath.ToSlash(relPath), nil
		} else if !os.IsNotExist(err) {
			return "", "", fmt.Errorf("stat existing parent: %w", err)
		}

		base := filepath.Base(existingParent)
		nextParent := filepath.Dir(existingParent)
		if nextParent == existingParent {
			return "", "", fmt.Errorf("path %q is outside the configured workspace", path)
		}

		missingParts = append([]string{base}, missingParts...)
		existingParent = nextParent
	}
}

func (g *WorkspaceGuard) ResolveScope(path string) (string, string, error) {
	if strings.TrimSpace(path) == "" {
		return g.root, ".", nil
	}
	return g.resolvePath(path)
}

func (g *WorkspaceGuard) resolvePath(path string) (string, string, error) {
	if strings.TrimSpace(path) == "" {
		return "", "", fmt.Errorf("path must be provided")
	}

	candidate := filepath.Clean(path)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(g.root, candidate)
	}

	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve path symlinks: %w", err)
	}

	if !g.contains(resolvedPath) {
		return "", "", fmt.Errorf("path %q is outside the configured workspace", path)
	}

	relPath, err := filepath.Rel(g.root, resolvedPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve relative path: %w", err)
	}
	if relPath == "." {
		return resolvedPath, ".", nil
	}

	return resolvedPath, filepath.ToSlash(relPath), nil
}

func (g *WorkspaceGuard) ReadTextFile(path string, limit int) ([]byte, bool, error) {
	if limit <= 0 || limit > maxFSTextFileBytes {
		limit = maxFSTextFileBytes
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, false, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, int64(limit+1)))
	if err != nil {
		return nil, false, fmt.Errorf("read file: %w", err)
	}

	truncated := len(data) > limit
	if truncated {
		data = data[:limit]
	}

	if isBinaryContent(data) {
		return nil, false, fmt.Errorf("file appears to be binary")
	}

	return data, truncated, nil
}

func normalizeFileText(data []byte) string {
	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func splitNormalizedLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func clampResultLimit(requested, fallback, max int) int {
	switch {
	case requested <= 0:
		return fallback
	case requested > max:
		return max
	default:
		return requested
	}
}

func clampLineCount(requested int) int {
	switch {
	case requested <= 0:
		return defaultFSReadLines
	case requested > maxFSReadLines:
		return maxFSReadLines
	default:
		return requested
	}
}

func getOptionalInt(args map[string]any, key string) int {
	value, ok := args[key]
	if !ok {
		return 0
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func getOptionalBool(args map[string]any, key string) bool {
	value, ok := args[key]
	if !ok {
		return false
	}

	typed, ok := value.(bool)
	if !ok {
		return false
	}

	return typed
}

func (g *WorkspaceGuard) contains(path string) bool {
	comparePath := normalizeComparisonPath(path)
	return comparePath == g.compareRoot || strings.HasPrefix(comparePath, g.compareRoot+string(os.PathSeparator))
}

func normalizeComparisonPath(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		clean = strings.ToLower(clean)
	}
	return clean
}

func isBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}

	return !utf8.Valid(data)
}
