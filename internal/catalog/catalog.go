package catalog

import (
	"encoding/json"
	"fmt"
	"strings"

)

// Catalog represents the documentation catalog structure.
type Catalog struct {
	Items []CatalogItem `json:"items"`
}

// CatalogItem represents a single item in the catalog tree.
type CatalogItem struct {
	Title    string        `json:"title"`
	Path     string        `json:"path"`
	Order    int           `json:"order"`
	Children []CatalogItem `json:"children"`
}

// FlatItem represents a flattened catalog item with computed paths.
type FlatItem struct {
	Title      string
	Path       string // dot-notation path, e.g., "core-modules.authentication"
	DirPath    string // filesystem path, e.g., "core-modules/authentication"
	Depth      int
	ParentPath string
	HasChildren bool
}

// Parse parses a JSON string into a Catalog.
func Parse(data string) (*Catalog, error) {
	var cat Catalog
	if err := json.Unmarshal([]byte(data), &cat); err != nil {
		return nil, fmt.Errorf("%s: %w", "failed to parse catalog JSON", err)
	}

	if len(cat.Items) == 0 {
		return nil, fmt.Errorf("%s", "catalog cannot be empty")
	}

	return &cat, nil
}

// Flatten returns all catalog items in depth-first order.
func (c *Catalog) Flatten() []FlatItem {
	var items []FlatItem
	for _, item := range c.Items {
		flattenItem(&items, item, "", 0)
	}
	return items
}

func flattenItem(items *[]FlatItem, item CatalogItem, parentPath string, depth int) {
	// Handle both formats:
	// Format A: child.Path = "introduction" (relative, needs parent prefix)
	// Format B: child.Path = "overview/introduction" (already includes parent)
	path := item.Path
	dirPath := strings.ReplaceAll(path, ".", "/")

	// If child path already contains a "/" it's likely already a full path (Format B)
	// If parentPath is set and the child path doesn't already start with parent prefix, prepend it
	if parentPath != "" {
		parentDir := strings.ReplaceAll(parentPath, ".", "/")
		if !strings.HasPrefix(dirPath, parentDir+"/") {
			path = parentPath + "." + item.Path
			dirPath = strings.ReplaceAll(path, ".", "/")
		} else {
			// path already includes parent, convert to dot-notation for consistency
			path = strings.ReplaceAll(dirPath, "/", ".")
		}
	}

	*items = append(*items, FlatItem{
		Title:       item.Title,
		Path:        path,
		DirPath:     dirPath,
		Depth:       depth,
		ParentPath:  parentPath,
		HasChildren: len(item.Children) > 0,
	})

	for _, child := range item.Children {
		flattenItem(items, child, path, depth+1)
	}
}

// ToJSON serializes the catalog to indented JSON.
func (c *Catalog) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TopLevelItems returns only the top-level items (no children).
func (c *Catalog) TopLevelItems() []CatalogItem {
	return c.Items
}

// BuildLinkTable returns a formatted string showing all catalog items
// and their corresponding directory paths, for use in prompts.
func (c *Catalog) BuildLinkTable() string {
	items := c.Flatten()
	var sb strings.Builder
	for _, item := range items {
		indent := strings.Repeat("  ", item.Depth)
		sb.WriteString(fmt.Sprintf("%s- 「%s」 → %s/index.md\n", indent, item.Title, item.DirPath))
	}
	return sb.String()
}
