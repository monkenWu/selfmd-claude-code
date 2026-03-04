package prompt

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*/*.tmpl templates/*.tmpl
var templateFS embed.FS

// Engine renders prompt templates with context data.
type Engine struct {
	templates       *template.Template // language-specific templates
	sharedTemplates *template.Template // shared templates (translate.tmpl)
}

// NewEngine creates a new prompt template engine for the given template language.
// templateLang should be a language code matching a subfolder under templates/ (e.g., "zh-TW", "en-US").
func NewEngine(templateLang string) (*Engine, error) {
	langGlob := fmt.Sprintf("templates/%s/*.tmpl", templateLang)
	tmpl, err := template.New("").ParseFS(templateFS, langGlob)
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt templates (%s): %w", templateLang, err)
	}

	shared, err := template.New("").ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to load shared templates: %w", err)
	}

	return &Engine{
		templates:       tmpl,
		sharedTemplates: shared,
	}, nil
}

// CatalogPromptData holds data for the catalog generation prompt.
type CatalogPromptData struct {
	RepositoryName       string
	ProjectType          string
	Language             string
	LanguageName         string // native display name (e.g., "繁體中文")
	LanguageOverride     bool   // true when template lang != output lang
	LanguageOverrideName string // native name of the desired output language
	KeyFiles             string
	EntryPoints          string
	FileTree             string
	ReadmeContent        string
}

// ContentPromptData holds data for content page generation.
type ContentPromptData struct {
	RepositoryName       string
	Language             string
	LanguageName         string
	LanguageOverride     bool
	LanguageOverrideName string
	CatalogPath          string
	CatalogTitle         string
	CatalogDirPath       string // filesystem dir path of THIS item, e.g., "configuration/claude-config"
	ProjectDir           string
	FileTree             string
	CatalogTable         string // formatted table of all catalog items with their dir paths
	ExistingContent      string // existing page content for update context (empty for new pages)
}

// UpdaterPromptData holds data for incremental update prompts (legacy, kept for reference).
type UpdaterPromptData struct {
	RepositoryName  string
	Language        string
	PreviousCommit  string
	CurrentCommit   string
	ChangedFiles    string
	ExistingCatalog string
	ExistingDocs    string
}

// UpdateMatchedPromptData holds data for deciding which existing pages need regeneration.
type UpdateMatchedPromptData struct {
	RepositoryName string
	Language       string
	ChangedFiles   string // list of changed source files
	AffectedPages  string // pages that reference these files (path + title + summary)
}

// UpdateUnmatchedPromptData holds data for deciding whether new pages are needed.
type UpdateUnmatchedPromptData struct {
	RepositoryName  string
	Language        string
	UnmatchedFiles  string // changed files not referenced in any existing doc
	ExistingCatalog string // existing catalog JSON
	CatalogTable    string // formatted link table of all pages
}

// TranslatePromptData holds data for translating a documentation page.
type TranslatePromptData struct {
	SourceLanguage     string // e.g., "zh-TW"
	SourceLanguageName string // e.g., "繁體中文"
	TargetLanguage     string // e.g., "en-US"
	TargetLanguageName string // e.g., "English"
	SourceContent      string // the full markdown content to translate
}

// TranslateTitlesPromptData holds data for batch-translating category titles.
type TranslateTitlesPromptData struct {
	SourceLanguage     string
	SourceLanguageName string
	TargetLanguage     string
	TargetLanguageName string
	Titles             []string
}

// RenderCatalog renders the catalog generation prompt.
func (e *Engine) RenderCatalog(data CatalogPromptData) (string, error) {
	return e.render("catalog.tmpl", data)
}

// RenderContent renders a content page generation prompt.
func (e *Engine) RenderContent(data ContentPromptData) (string, error) {
	return e.render("content.tmpl", data)
}

// RenderUpdater renders the incremental update prompt (legacy).
func (e *Engine) RenderUpdater(data UpdaterPromptData) (string, error) {
	return e.render("updater.tmpl", data)
}

// RenderUpdateMatched renders the prompt for deciding which existing pages need regeneration.
func (e *Engine) RenderUpdateMatched(data UpdateMatchedPromptData) (string, error) {
	return e.render("update_matched.tmpl", data)
}

// RenderUpdateUnmatched renders the prompt for deciding whether new pages are needed.
func (e *Engine) RenderUpdateUnmatched(data UpdateUnmatchedPromptData) (string, error) {
	return e.render("update_unmatched.tmpl", data)
}

// RenderTranslate renders the translation prompt.
func (e *Engine) RenderTranslate(data TranslatePromptData) (string, error) {
	return e.renderShared("translate.tmpl", data)
}

// RenderTranslateTitles renders the batch title translation prompt.
func (e *Engine) RenderTranslateTitles(data TranslateTitlesPromptData) (string, error) {
	return e.renderShared("translate_titles.tmpl", data)
}

func (e *Engine) render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("failed to render template %s: %w", name, err)
	}
	return buf.String(), nil
}

func (e *Engine) renderShared(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := e.sharedTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("failed to render shared template %s: %w", name, err)
	}
	return buf.String(), nil
}
