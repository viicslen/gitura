// Package runner provides utilities for executing user-configured CLI commands
// against PR comment text, capturing their output for display in the gitura UI.
package runner

import (
	"bytes"
	"context"
	"errors"
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
		CommandName: cmd.Name,
		Input:       input,
		StartedAt:   startedAt.Format(time.RFC3339),
	}

	usePlaceholder := strings.Contains(cmd.Command, placeholderToken)
	rawCmd := buildRawCommand(cmd.Command, input, localPath, usePlaceholder)

	argv, err := parseArgv(rawCmd)
	if err != nil {
		return parseFailureResult(result, err)
	}

	//nolint:gosec // argv comes from user-configured command strings — this is intentional.
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)

	var stdoutBuf, stderrBuf bytes.Buffer
	c.Stdout = &stdoutBuf
	c.Stderr = &stderrBuf
	configureCommand(c, input, localPath, usePlaceholder)

	runErr := c.Run()
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	applyRunResult(&result, ctx, runErr)

	return result
}

func buildRawCommand(command string, input string, localPath string, usePlaceholder bool) string {
	rawCmd := command
	if localPath != "" {
		rawCmd = strings.ReplaceAll(rawCmd, repoPathToken, shellEscape(localPath))
	}
	if usePlaceholder {
		rawCmd = strings.ReplaceAll(rawCmd, placeholderToken, shellEscape(input))
	}

	return rawCmd
}

var errEmptyCommand = errors.New("empty command")

func parseArgv(rawCmd string) ([]string, error) {
	argv, err := shellSplit(rawCmd)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, errEmptyCommand
	}

	return argv, nil
}

func parseFailureResult(result model.RunResult, err error) model.RunResult {
	result.Stderr = fmt.Sprintf("runner: failed to parse command: %v", err)
	result.ExitCode = -1
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)

	return result
}

func configureCommand(command *exec.Cmd, input string, localPath string, usePlaceholder bool) {
	if localPath != "" {
		command.Dir = localPath
	}
	if !usePlaceholder {
		command.Stdin = strings.NewReader(input)
	}
}

func applyRunResult(result *model.RunResult, ctx context.Context, runErr error) {
	if ctx.Err() != nil {
		result.Cancelled = true
		result.ExitCode = -1

		return
	}
	if runErr == nil {
		return
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()

		return
	}

	result.ExitCode = -1
	if result.Stderr == "" {
		result.Stderr = runErr.Error()
	}
}

type shellSplitState struct {
	args     []string
	current  strings.Builder
	inSingle bool
	inDouble bool
}

// shellSplit splits a command string into an argv slice using simple POSIX
// shell quoting rules (single-quoted, double-quoted, and unquoted tokens).
// No variable expansion or glob expansion is performed.
func shellSplit(s string) ([]string, error) {
	state := shellSplitState{}

	for i := 0; i < len(s); i++ {
		processShellChar(&state, s, &i)
	}

	if state.inSingle {
		return nil, fmt.Errorf("unterminated single quote")
	}
	if state.inDouble {
		return nil, fmt.Errorf("unterminated double quote")
	}
	flushCurrentArg(&state)

	return state.args, nil
}

func processShellChar(state *shellSplitState, s string, i *int) {
	ch := rune(s[*i])

	if state.inSingle {
		processSingleQuotedChar(state, ch)

		return
	}
	if state.inDouble {
		processDoubleQuotedChar(state, s, i, ch)

		return
	}

	processUnquotedChar(state, ch)
}

func processSingleQuotedChar(state *shellSplitState, ch rune) {
	if ch == '\'' {
		state.inSingle = false

		return
	}

	state.current.WriteRune(ch)
}

func processDoubleQuotedChar(state *shellSplitState, s string, i *int, ch rune) {
	if ch == '"' {
		state.inDouble = false

		return
	}

	if ch == '\\' {
		if consumeDoubleQuoteEscape(state, s, i) {
			return
		}
	}

	state.current.WriteRune(ch)
}

func consumeDoubleQuoteEscape(state *shellSplitState, s string, i *int) bool {
	if *i+1 >= len(s) {
		return false
	}

	next := rune(s[*i+1])
	if next != '"' && next != '\\' && next != '$' && next != '`' {
		return false
	}

	state.current.WriteRune(next)
	*i++

	return true
}

func processUnquotedChar(state *shellSplitState, ch rune) {
	switch {
	case ch == '\'':
		state.inSingle = true
	case ch == '"':
		state.inDouble = true
	case unicode.IsSpace(ch):
		flushCurrentArg(state)
	default:
		state.current.WriteRune(ch)
	}
}

func flushCurrentArg(state *shellSplitState) {
	if state.current.Len() == 0 {
		return
	}

	state.args = append(state.args, state.current.String())
	state.current.Reset()
}

// shellEscape wraps s in single quotes, escaping any embedded single quotes
// using the standard POSIX trick ('\”).
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
