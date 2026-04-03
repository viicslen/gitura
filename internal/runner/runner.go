// Package runner provides utilities for executing user-configured CLI commands
// against PR comment text, capturing their output for display in the gitura UI.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"gitura/internal/model"
)

// placeholderToken is the literal string the user places in a command template
// to indicate where the input text should be injected as a shell argument
// instead of being piped via stdin.
const placeholderToken = "{{instructions}}"

// repoPathToken is the literal string the user places in a command template
// to indicate where the local repository path should be injected.
const repoPathToken = "{{repo_path}}"

// RunCommand executes cmd against input and returns the result.
//
// Input delivery strategy:
//   - If cmd.Command contains "{{instructions}}", the placeholder is replaced
//     with input as a single shell argument; stdin is left empty.
//   - Otherwise, input is written to the process's stdin pipe.
//
// If localPath is non-empty, it is used as the working directory for the
// subprocess and is also substituted for any "{{repo_path}}" token in the
// command string before argv-splitting.
//
// The command string is POSIX-shell-split (respects quoted tokens) to produce
// the argv slice; no shell interpreter is invoked, preventing injection.
//
// If ctx is cancelled before the process finishes, the process is killed and
// the returned RunResult has Cancelled=true.
func RunCommand(ctx context.Context, cmd model.CommandDTO, input string, localPath string) model.RunResult {
	startedAt := time.Now().UTC()

	result := model.RunResult{
		CommandID:   cmd.ID,
		CommandName: cmd.Name,
		Input:       input,
		StartedAt:   startedAt.Format(time.RFC3339),
	}

	usePlaceholder := strings.Contains(cmd.Command, placeholderToken)

	// Build argv by substituting the placeholder or leaving it unchanged.
	rawCmd := cmd.Command
	if localPath != "" {
		rawCmd = strings.ReplaceAll(rawCmd, repoPathToken, shellEscape(localPath))
	}
	if usePlaceholder {
		rawCmd = strings.ReplaceAll(rawCmd, placeholderToken, shellEscape(input))
	}

	argv, err := shellSplit(rawCmd)
	if err != nil || len(argv) == 0 {
		result.Stderr = fmt.Sprintf("runner: failed to parse command: %v", err)
		result.ExitCode = -1
		result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		return result
	}

	//nolint:gosec // argv comes from user-configured command strings — this is intentional.
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)

	var stdoutBuf, stderrBuf bytes.Buffer
	c.Stdout = &stdoutBuf
	c.Stderr = &stderrBuf

	if localPath != "" {
		c.Dir = localPath
	}

	if !usePlaceholder {
		c.Stdin = strings.NewReader(input)
	}

	runErr := c.Run()
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)

	if ctx.Err() != nil {
		result.Cancelled = true
		result.ExitCode = -1
		return result
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			if result.Stderr == "" {
				result.Stderr = runErr.Error()
			}
		}
	}

	return result
}

// shellSplit splits a command string into an argv slice using simple POSIX
// shell quoting rules (single-quoted, double-quoted, and unquoted tokens).
// No variable expansion or glob expansion is performed.
func shellSplit(s string) ([]string, error) {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		ch := rune(s[i])
		switch {
		case inSingle:
			if ch == '\'' {
				inSingle = false
			} else {
				current.WriteRune(ch)
			}
		case inDouble:
			if ch == '"' {
				inDouble = false
			} else if ch == '\\' && i+1 < len(s) {
				next := rune(s[i+1])
				// Only honour backslash-escapes for a limited set inside double-quotes.
				if next == '"' || next == '\\' || next == '$' || next == '`' {
					current.WriteRune(next)
					i++
				} else {
					current.WriteRune(ch)
				}
			} else {
				current.WriteRune(ch)
			}
		case ch == '\'':
			inSingle = true
		case ch == '"':
			inDouble = true
		case unicode.IsSpace(ch):
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if inSingle {
		return nil, fmt.Errorf("unterminated single quote")
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

// shellEscape wraps s in single quotes, escaping any embedded single quotes
// using the standard POSIX trick ('\”).
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
