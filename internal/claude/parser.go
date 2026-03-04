package claude

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

)

// ParseResponse parses the JSON output from `claude -p --output-format json`.
func ParseResponse(data []byte) (*RunResult, error) {
	var resp CLIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	return &RunResult{
		Content:    resp.Result,
		IsError:    resp.IsError,
		DurationMs: resp.DurationMs,
		CostUSD:    resp.TotalCost,
		SessionID:  resp.SessionID,
	}, nil
}

// ExtractJSONBlock extracts the first JSON code block from markdown text.
// Looks for ```json ... ``` fenced blocks, then falls back to raw JSON.
func ExtractJSONBlock(text string) (string, error) {
	// try fenced code block first
	re := regexp.MustCompile("(?s)```json\\s*\n(.*?)```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}

	// try without language tag
	re = regexp.MustCompile("(?s)```\\s*\n(\\{.*?\\})\\s*```")
	matches = re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}

	// try to find raw JSON object
	start := strings.Index(text, "{")
	if start >= 0 {
		depth := 0
		for i := start; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return text[start : i+1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("%s", "failed to extract JSON block from response")
}

// ExtractMarkdown extracts markdown content from Claude's response.
// Strips wrapping fences and boilerplate messages if present.
func ExtractMarkdown(text string) string {
	text = strings.TrimSpace(text)

	// strip wrapping ```markdown ... ``` if present
	if strings.HasPrefix(text, "```markdown") {
		re := regexp.MustCompile("(?s)^```markdown\\s*\n(.*?)```\\s*$")
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			text = strings.TrimSpace(matches[1])
		}
	}

	// strip wrapping ```md ... ``` if present
	if strings.HasPrefix(text, "```md") {
		re := regexp.MustCompile("(?s)^```md\\s*\n(.*?)```\\s*$")
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			text = strings.TrimSpace(matches[1])
		}
	}

	// Remove boilerplate messages when Claude tried to write files but was denied
	text = cleanBoilerplate(text)

	return text
}

// ExtractDocumentTag extracts content from <document>...</document> XML tags.
// If multiple <document> blocks exist, returns the longest one.
func ExtractDocumentTag(text string) (string, error) {
	re := regexp.MustCompile("(?s)<document>\\s*\\n?(.*?)\\s*</document>")
	allMatches := re.FindAllStringSubmatch(text, -1)
	if len(allMatches) == 0 {
		return "", fmt.Errorf("%s", "failed to extract <document> tag content from response")
	}

	// Pick the longest match (most likely the real content, not an example)
	best := ""
	for _, m := range allMatches {
		content := strings.TrimSpace(m[1])
		if len(content) > len(best) {
			best = content
		}
	}

	if best == "" {
		return "", fmt.Errorf("%s", "failed to extract <document> tag content from response")
	}
	return best, nil
}

// cleanBoilerplate removes trailing boilerplate messages that Claude appends
// when it fails to write files (permission denied).
func cleanBoilerplate(text string) string {
	// Common boilerplate patterns at the end of the response (Chinese + English)
	markers := []string{
		// Chinese
		"文件已完成撰寫",
		"文件已完成輸出",
		"文件已完成產生",
		"由於寫入",
		"的檔案權限未被授予",
		"完整的 Markdown 文件內容已在上方直接輸出",
		"文件涵蓋了",
		"所有程式碼範例均附有來源標註",
		"相關連結使用正確的相對路徑格式",
		// English
		"documentation has been completed",
		"document has been written",
		"document has been generated",
		"file write permission was denied",
		"the complete Markdown content has been output above",
		"all code examples are annotated with source",
		"relative path links are correctly formatted",
	}

	// Find the earliest marker that appears after the main content
	// Main content should end with the last markdown section
	lines := strings.Split(text, "\n")
	cutIdx := len(lines)

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		isBoilerplate := false
		for _, marker := range markers {
			if strings.Contains(strings.ToLower(line), strings.ToLower(marker)) {
				isBoilerplate = true
				break
			}
		}
		if isBoilerplate {
			cutIdx = i
		} else {
			break
		}
	}

	if cutIdx < len(lines) {
		text = strings.TrimSpace(strings.Join(lines[:cutIdx], "\n"))
	}

	return text
}
