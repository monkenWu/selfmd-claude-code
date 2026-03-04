package claude

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/monkenwu/selfmd/internal/config"
)

// Runner manages Claude CLI subprocess invocations.
type Runner struct {
	config *config.ClaudeConfig
	logger *slog.Logger
}

// NewRunner creates a new Claude CLI runner.
func NewRunner(cfg *config.ClaudeConfig, logger *slog.Logger) *Runner {
	return &Runner{
		config: cfg,
		logger: logger,
	}
}

// Run executes a single Claude CLI invocation.
func (r *Runner) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	// build command args
	args := []string{
		"-p",
		"--output-format", "json",
	}

	model := opts.Model
	if model == "" {
		model = r.config.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	tools := opts.AllowedTools
	if len(tools) == 0 {
		tools = r.config.AllowedTools
	}
	if len(tools) > 0 {
		for _, t := range tools {
			args = append(args, "--allowedTools", t)
		}
	}

	// Explicitly block Write/Edit to prevent content from being lost in denied tool calls
	args = append(args, "--disallowedTools", "Write", "--disallowedTools", "Edit")

	args = append(args, r.config.ExtraArgs...)
	args = append(args, opts.ExtraArgs...)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = time.Duration(r.config.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	// pipe prompt via stdin
	cmd.Stdin = strings.NewReader(opts.Prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	r.logger.Debug("invoking claude", "work_dir", opts.WorkDir, "model", model)

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("Claude CLI execution timed out (%v)", timeout)
		}
		stderrStr := stderr.String()
		if stderrStr != "" {
			return nil, fmt.Errorf("Claude CLI execution failed: %w\nstderr: %s", err, stderrStr)
		}
		return nil, fmt.Errorf("Claude CLI execution failed: %w", err)
	}

	result, err := ParseResponse(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %w\nraw output: %s", err, stdout.String()[:min(500, stdout.Len())])
	}

	r.logger.Debug("claude completed",
		"duration", elapsed.Round(time.Millisecond),
		"cost_usd", result.CostUSD,
		"is_error", result.IsError,
	)

	return result, nil
}

// RunWithRetry executes a Claude CLI invocation with retry logic.
func (r *Runner) RunWithRetry(ctx context.Context, opts RunOptions) (*RunResult, error) {
	maxRetries := r.config.MaxRetries
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 5 * time.Second
			r.logger.Info("retrying", "attempt", attempt+1, "backoff", backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := r.Run(ctx, opts)
		if err == nil && !result.IsError {
			return result, nil
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("Claude reported error: %s", result.Content)
		}

		r.logger.Warn("Claude call failed", "attempt", attempt+1, "error", lastErr)
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxRetries+1, lastErr)
}

// CheckAvailable verifies that the claude CLI is installed and accessible.
func CheckAvailable() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found. Please install Claude Code: https://docs.anthropic.com/en/docs/claude-code")
	}
	return nil
}
