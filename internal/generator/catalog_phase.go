package generator

import (
	"context"
	"fmt"

	"github.com/monkenwu/selfmd/internal/catalog"
	"github.com/monkenwu/selfmd/internal/claude"
	"github.com/monkenwu/selfmd/internal/config"
	"github.com/monkenwu/selfmd/internal/prompt"
	"github.com/monkenwu/selfmd/internal/scanner"
)

// GenerateCatalog invokes Claude to generate the documentation catalog.
func (g *Generator) GenerateCatalog(ctx context.Context, scan *scanner.ScanResult) (*catalog.Catalog, error) {
	langName := config.GetLangNativeName(g.Config.Output.Language)
	data := prompt.CatalogPromptData{
		RepositoryName:       g.Config.Project.Name,
		ProjectType:          g.Config.Project.Type,
		Language:             g.Config.Output.Language,
		LanguageName:         langName,
		LanguageOverride:     g.Config.Output.NeedsLanguageOverride(),
		LanguageOverrideName: langName,
		KeyFiles:             scan.KeyFiles(),
		EntryPoints:          scan.EntryPointsFormatted(),
		FileTree:             scanner.RenderTree(scan.Tree, 4),
		ReadmeContent:        scan.ReadmeContent,
	}

	rendered, err := g.Engine.RenderCatalog(data)
	if err != nil {
		return nil, err
	}

	fmt.Print("      Calling Claude to generate catalog...")

	result, err := g.Runner.RunWithRetry(ctx, claude.RunOptions{
		Prompt:  rendered,
		WorkDir: g.RootDir,
	})
	if err != nil {
		fmt.Println(" Failed")
		return nil, err
	}

	g.TotalCost += result.CostUSD
	fmt.Printf(" Done (%.1fs, $%.4f)\n", float64(result.DurationMs)/1000, result.CostUSD)

	// Extract JSON from response
	jsonStr, err := claude.ExtractJSONBlock(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract catalog JSON from Claude response: %w", err)
	}

	cat, err := catalog.Parse(jsonStr)
	if err != nil {
		return nil, err
	}

	return cat, nil
}
