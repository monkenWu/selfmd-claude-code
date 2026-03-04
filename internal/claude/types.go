package claude

import "time"

// RunOptions configures a single Claude CLI invocation.
type RunOptions struct {
	Prompt       string
	WorkDir      string        // CWD for the claude process
	AllowedTools []string      // tool restrictions
	Model        string        // model override
	Timeout      time.Duration // per-invocation timeout
	ExtraArgs    []string      // additional CLI arguments
}

// RunResult holds the parsed result from a Claude CLI invocation.
type RunResult struct {
	Content    string  // the text result from Claude
	IsError    bool    // whether Claude reported an error
	DurationMs int64   // execution time in milliseconds
	CostUSD    float64 // cost of this invocation
	SessionID  string  // Claude session ID
}

// CLIResponse represents the JSON response from `claude -p --output-format json`.
type CLIResponse struct {
	Type       string  `json:"type"`
	Subtype    string  `json:"subtype"`
	IsError    bool    `json:"is_error"`
	Result     string  `json:"result"`
	DurationMs int64   `json:"duration_ms"`
	TotalCost  float64 `json:"total_cost_usd"`
	SessionID  string  `json:"session_id"`
}
