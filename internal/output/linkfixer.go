package output

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/monkenwu/selfmd/internal/catalog"
)

// LinkFixer validates and fixes relative links in generated markdown content.
type LinkFixer struct {
	allItems  []catalog.FlatItem
	dirPaths  map[string]bool   // set of all valid dirPaths
	pathIndex map[string]string // various lookup keys → dirPath
}

// NewLinkFixer creates a link fixer from a catalog.
func NewLinkFixer(cat *catalog.Catalog) *LinkFixer {
	items := cat.Flatten()
	dirPaths := make(map[string]bool)
	pathIndex := make(map[string]string)

	for _, item := range items {
		dirPaths[item.DirPath] = true

		// index by multiple keys for fuzzy matching
		pathIndex[item.DirPath] = item.DirPath
		pathIndex[item.Path] = item.DirPath                          // dot-notation
		pathIndex[strings.ReplaceAll(item.Path, ".", "/")] = item.DirPath // explicit slash conversion

		// index by last segment (e.g., "scanner" → "core-modules/scanner")
		parts := strings.Split(item.DirPath, "/")
		lastSeg := parts[len(parts)-1]
		if _, exists := pathIndex[lastSeg]; !exists {
			pathIndex[lastSeg] = item.DirPath
		}

		// index by slug-like variations
		pathIndex[strings.ToLower(item.DirPath)] = item.DirPath
	}

	return &LinkFixer{
		allItems:  items,
		dirPaths:  dirPaths,
		pathIndex: pathIndex,
	}
}

// FixLinks scans markdown content for relative links and fixes broken ones.
// currentDirPath is the DirPath of the page being processed (e.g., "configuration/claude-config").
func (lf *LinkFixer) FixLinks(content string, currentDirPath string) string {
	// match markdown links: [text](target)
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	return linkRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		text := parts[1]
		target := parts[2]

		// skip external links, anchors, and absolute paths
		if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") {
			return match
		}

		// skip image links
		if strings.HasPrefix(match, "![") {
			return match
		}

		// try to fix the link
		fixed := lf.fixSingleLink(target, currentDirPath)
		if fixed != "" && fixed != target {
			return "[" + text + "](" + fixed + ")"
		}

		return match
	})
}

func (lf *LinkFixer) fixSingleLink(target, currentDirPath string) string {
	// already a valid relative link pointing to existing page?
	resolved := lf.resolveRelative(target, currentDirPath)
	if resolved != "" && lf.isValidTarget(resolved) {
		return target // already correct
	}

	// extract the "meaningful" part from the target
	cleaned := lf.extractPathSlug(target)
	if cleaned == "" {
		return ""
	}

	// look up in index
	targetDirPath, found := lf.pathIndex[cleaned]
	if !found {
		// try lowercase
		targetDirPath, found = lf.pathIndex[strings.ToLower(cleaned)]
	}
	if !found {
		// try last segment only
		segments := strings.FieldsFunc(cleaned, func(r rune) bool {
			return r == '.' || r == '/'
		})
		if len(segments) > 0 {
			last := segments[len(segments)-1]
			targetDirPath, found = lf.pathIndex[last]
		}
		// try combining last two segments with hyphen (e.g., "prompt/engine" → "prompt-engine")
		if !found && len(segments) >= 2 {
			hyphenated := segments[len(segments)-2] + "-" + segments[len(segments)-1]
			targetDirPath, found = lf.pathIndex[hyphenated]
		}
	}
	if !found {
		// substring match: find any dirPath that ends with the cleaned slug
		for dp := range lf.dirPaths {
			if strings.HasSuffix(dp, "/"+cleaned) || strings.HasSuffix(dp, "/"+strings.ReplaceAll(cleaned, "/", "-")) {
				targetDirPath = dp
				found = true
				break
			}
		}
	}
	if !found {
		return "" // can't fix
	}

	// compute correct relative path from currentDirPath to targetDirPath
	return lf.computeRelativePath(currentDirPath, targetDirPath)
}

// extractPathSlug strips common noise from a broken link target.
func (lf *LinkFixer) extractPathSlug(target string) string {
	// strip trailing /index.md or /
	target = strings.TrimSuffix(target, "/index.md")
	target = strings.TrimSuffix(target, "/")
	target = strings.TrimSuffix(target, ".md")

	// strip leading ../
	for strings.HasPrefix(target, "../") {
		target = strings.TrimPrefix(target, "../")
	}
	target = strings.TrimPrefix(target, "./")

	// convert dots to slashes for dot-notation
	if strings.Contains(target, ".") && !strings.Contains(target, "/") {
		target = strings.ReplaceAll(target, ".", "/")
	}

	return strings.TrimSpace(target)
}

// resolveRelative resolves a relative link target from the current page's directory.
func (lf *LinkFixer) resolveRelative(target, currentDirPath string) string {
	// compute what dirPath the target would point to
	cleaned := strings.TrimSuffix(target, "/index.md")
	cleaned = strings.TrimSuffix(cleaned, "/")

	joined := filepath.Join(currentDirPath, cleaned)
	joined = filepath.ToSlash(filepath.Clean(joined))

	return joined
}

// isValidTarget checks if a dirPath corresponds to a real catalog item.
func (lf *LinkFixer) isValidTarget(dirPath string) bool {
	return lf.dirPaths[dirPath]
}

// computeRelativePath computes the relative path from one catalog item to another.
func (lf *LinkFixer) computeRelativePath(fromDirPath, toDirPath string) string {
	rel, err := filepath.Rel(fromDirPath, toDirPath)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	return rel + "/index.md"
}
