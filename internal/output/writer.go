package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monkenwu/selfmd/internal/catalog"
)

// DocMeta holds metadata about the documentation build, including language info.
type DocMeta struct {
	DefaultLanguage    string     `json:"default_language"`
	AvailableLanguages []LangInfo `json:"available_languages"`
}

// LangInfo describes a single available language.
type LangInfo struct {
	Code       string `json:"code"`
	NativeName string `json:"native_name"`
	IsDefault  bool   `json:"is_default"`
}

// Writer handles writing documentation files to the output directory.
type Writer struct {
	BaseDir string // absolute path to .doc-build/
}

// NewWriter creates a new output writer.
func NewWriter(baseDir string) *Writer {
	return &Writer{BaseDir: baseDir}
}

// Clean removes the output directory and recreates it.
func (w *Writer) Clean() error {
	if err := os.RemoveAll(w.BaseDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %w", err)
	}
	return os.MkdirAll(w.BaseDir, 0755)
}

// EnsureDir ensures the output directory exists.
func (w *Writer) EnsureDir() error {
	return os.MkdirAll(w.BaseDir, 0755)
}

// WritePage writes a documentation page for a catalog item.
func (w *Writer) WritePage(item catalog.FlatItem, content string) error {
	dir := filepath.Join(w.BaseDir, item.DirPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	path := filepath.Join(dir, "index.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// WriteFile writes a file with the given relative path under the output directory.
func (w *Writer) WriteFile(relPath string, content string) error {
	path := filepath.Join(w.BaseDir, relPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// WriteCatalogJSON saves the catalog as JSON for incremental updates.
func (w *Writer) WriteCatalogJSON(cat *catalog.Catalog) error {
	data, err := cat.ToJSON()
	if err != nil {
		return err
	}
	return w.WriteFile("_catalog.json", data)
}

// ReadCatalogJSON reads the saved catalog JSON.
func (w *Writer) ReadCatalogJSON() (string, error) {
	path := filepath.Join(w.BaseDir, "_catalog.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read catalog JSON: %w", err)
	}
	return string(data), nil
}

// PageExists checks if a documentation page exists and has valid content.
// Returns false if the file doesn't exist, is empty, or contains placeholder/failed content.
func (w *Writer) PageExists(item catalog.FlatItem) bool {
	path := filepath.Join(w.BaseDir, item.DirPath, "index.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return false
	}
	// Only check the first 500 bytes for the failure marker to avoid false positives
	// when translated docs reference the marker string in their content.
	head := content
	if len(head) > 500 {
		head = head[:500]
	}
	if strings.Contains(head, "This page failed to generate") {
		return false
	}
	return true
}

// ReadPage reads the content of a documentation page.
func (w *Writer) ReadPage(item catalog.FlatItem) (string, error) {
	path := filepath.Join(w.BaseDir, item.DirPath, "index.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveLastCommit saves the current commit hash for incremental updates.
func (w *Writer) SaveLastCommit(commit string) error {
	return w.WriteFile("_last_commit", commit)
}

// ReadLastCommit reads the saved commit hash.
func (w *Writer) ReadLastCommit() (string, error) {
	path := filepath.Join(w.BaseDir, "_last_commit")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read last commit: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ForLanguage returns a new Writer that writes to a language-specific subdirectory.
func (w *Writer) ForLanguage(lang string) *Writer {
	return &Writer{
		BaseDir: filepath.Join(w.BaseDir, lang),
	}
}
