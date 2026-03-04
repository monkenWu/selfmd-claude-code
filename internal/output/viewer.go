package output

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

)

//go:embed viewer/index.html
var viewerHTML string

//go:embed viewer/app.js
var viewerJS string

//go:embed viewer/style.css
var viewerCSS string

// WriteViewer writes the static documentation viewer (HTML/JS/CSS) to the output directory
// and bundles all generated markdown content into _data.js for offline/serverless viewing.
func (w *Writer) WriteViewer(projectName string, docMeta *DocMeta) error {
	// Write index.html with project name and language injected
	html := strings.ReplaceAll(viewerHTML, "{{PROJECT_NAME}}", projectName)
	lang := "zh-TW"
	if docMeta != nil {
		lang = docMeta.DefaultLanguage
	}
	html = strings.ReplaceAll(html, "{{LANG}}", lang)

	if err := w.WriteFile("index.html", html); err != nil {
		return fmt.Errorf("failed to write index.html: %w", err)
	}

	// Write static assets
	if err := w.WriteFile("app.js", viewerJS); err != nil {
		return fmt.Errorf("failed to write app.js: %w", err)
	}
	if err := w.WriteFile("style.css", viewerCSS); err != nil {
		return fmt.Errorf("failed to write style.css: %w", err)
	}

	// Write _doc_meta.json
	if docMeta != nil {
		metaBytes, err := json.MarshalIndent(docMeta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize _doc_meta.json: %w", err)
		}
		if err := w.WriteFile("_doc_meta.json", string(metaBytes)); err != nil {
			return fmt.Errorf("failed to write _doc_meta.json: %w", err)
		}
	}

	// Bundle all content into _data.js
	return w.bundleData(projectName, docMeta)
}

// bundleData walks the output directory, collects all .md files and _catalog.json,
// and writes them as a single _data.js file for client-side rendering.
func (w *Writer) bundleData(projectName string, docMeta *DocMeta) error {
	// Read catalog
	catalogPath := filepath.Join(w.BaseDir, "_catalog.json")
	catalogBytes, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to read _catalog.json: %w", err)
	}

	var catalogObj interface{}
	if err := json.Unmarshal(catalogBytes, &catalogObj); err != nil {
		return fmt.Errorf("failed to parse _catalog.json: %w", err)
	}

	// Collect all master-language .md files (skip language subdirectories)
	pages := make(map[string]string)
	langDirs := make(map[string]bool)
	if docMeta != nil {
		for _, lang := range docMeta.AvailableLanguages {
			if !lang.IsDefault {
				langDirs[lang.Code] = true
			}
		}
	}

	err = filepath.Walk(w.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		relPath, err := filepath.Rel(w.BaseDir, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Skip files starting with _
		if strings.HasPrefix(filepath.Base(relPath), "_") {
			return nil
		}

		// Skip files inside language subdirectories
		topDir := strings.SplitN(relPath, "/", 2)[0]
		if langDirs[topDir] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		pages[relPath] = string(content)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan .md files: %w", err)
	}

	// Build data object
	data := map[string]interface{}{
		"catalog": catalogObj,
		"pages":   pages,
	}

	// Add language metadata and secondary language data
	if docMeta != nil {
		data["meta"] = docMeta

		languages := make(map[string]interface{})
		for _, lang := range docMeta.AvailableLanguages {
			if lang.IsDefault {
				continue
			}
			langDir := filepath.Join(w.BaseDir, lang.Code)
			if _, statErr := os.Stat(langDir); os.IsNotExist(statErr) {
				continue
			}

			langEntry := make(map[string]interface{})

			// Read lang-specific catalog
			langCatalogPath := filepath.Join(langDir, "_catalog.json")
			if catBytes, readErr := os.ReadFile(langCatalogPath); readErr == nil {
				var catObj interface{}
				if json.Unmarshal(catBytes, &catObj) == nil {
					langEntry["catalog"] = catObj
				}
			}

			// Read lang-specific pages
			langPages := make(map[string]string)
			filepath.Walk(langDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
					return nil
				}
				relPath, err := filepath.Rel(langDir, path)
				if err != nil {
					return nil
				}
				relPath = filepath.ToSlash(relPath)
				if strings.HasPrefix(filepath.Base(relPath), "_") {
					return nil
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				langPages[relPath] = string(content)
				return nil
			})
			langEntry["pages"] = langPages

			languages[lang.Code] = langEntry
		}

		if len(languages) > 0 {
			data["languages"] = languages
		}
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize _data.js: %w", err)
	}

	content := "window.DOC_DATA = " + string(jsonBytes) + ";\n"
	return w.WriteFile("_data.js", content)
}

// Ensure embed import is used
var _ embed.FS
