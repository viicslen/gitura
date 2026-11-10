// Package runner provides utilities for executing user-configured CLI commands
// against PR comment text, capturing their output for display in the gitura UI.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode"

	"gitura/internal/model"
)

// instructionsToken is the literal string the user places in a command template
// to indicate where the input text should be injected as a shell argument
// instead of being piped via stdin.
const instructionsToken = "{{instructions}}"

// repoPathToken is the literal string the user places in a command template
// to indicate where the local repository path should be injected.
const repoPathToken = "{{repo_path}}"

// OutputChunkHandler receives raw output chunks as they are read.
// stream is either "stdout" or "stderr".
type OutputChunkHandler func(stream string, chunk string)

// Run executes cmd against input and returns the result.
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
// the argv slice; placeholders are then substituted into individual argv
// elements, so neither input nor localPath can introduce extra arguments. No
// shell interpreter is invoked.
//
// If ctx is cancelled before the process finishes, the process is killed and
// the returned RunResult has Cancelled=true.
// When onChunk is non-nil, raw stdout/stderr chunks are streamed as they are
// produced by the subprocess.
func Run(
	ctx context.Context,
	cmd model.CommandDTO,
	input string,
	localPath string,
	onChunk OutputChunkHandler,
) model.RunResult {
	startedAt := time.Now().UTC()

	result := model.RunResult{
		CommandName: cmd.Name,
		Input:       input,
		StartedAt:   startedAt.Format(time.RFC3339),
	}

	usePlaceholder := strings.Contains(cmd.Command, instructionsToken)

	argv, err := parseArgv(cmd.Command)
	if err != nil {
		return parseFailureResult(result, err)
	}
	argv = substitutePlaceholders(argv, input, localPath, usePlaceholder)

	// #nosec G204 -- argv comes from the user's own command config; untrusted input is substituted per-argument, never re-split.
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)

	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return startFailureResult(result, err)
	}
	stderrPipe, err := c.StderrPipe()
	if err != nil {
		return startFailureResult(result, err)
	}
	configureCommand(c, input, localPath, usePlaceholder)
	if err := c.Start(); err != nil {
		return startFailureResult(result, err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	streamErrs := make(chan error, 2)
	var streamWG sync.WaitGroup
	streamWG.Add(2)

	go readOutputStream(&streamWG, streamErrs, "stdout", stdoutPipe, &stdoutBuf, onChunk)
	go readOutputStream(&streamWG, streamErrs, "stderr", stderrPipe, &stderrBuf, onChunk)

	runErr := c.Wait()
	streamWG.Wait()
	close(streamErrs)
	for streamErr := range streamErrs {
		if streamErr != nil {
			stderrBuf.WriteString("\n")
			stderrBuf.WriteString(streamErr.Error())
		}
	}

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	applyRunResult(&result, ctx, runErr)

	return result
}

func startFailureResult(result model.RunResult, err error) model.RunResult {
	result.Stderr = fmt.Sprintf("runner: failed to start command: %v", err)
	result.ExitCode = -1
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)

	return result
}

func readOutputStream(
	wg *sync.WaitGroup,
	errCh chan<- error,
	stream string,
	r io.Reader,
	dst *bytes.Buffer,
	onChunk OutputChunkHandler,
) {
	defer wg.Done()
	errCh <- streamToBuffer(stream, r, dst, onChunk)
}

func streamToBuffer(
	stream string,
	r io.Reader,
	dst *bytes.Buffer,
	onChunk OutputChunkHandler,
) error {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			dst.WriteString(chunk)
			if onChunk != nil {
				onChunk(stream, chunk)
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("runner: failed reading %s: %w", stream, err)
	}
}

// substitutePlaceholders replaces the placeholder tokens within each argv
// element. Substituting after the split — rather than into the command string
// before it — keeps untrusted input confined to the argument it appears in.
func substitutePlaceholders(argv []string, input string, localPath string, usePlaceholder bool) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		if localPath != "" {
			arg = strings.ReplaceAll(arg, repoPathToken, localPath)
		}
		if usePlaceholder {
			arg = strings.ReplaceAll(arg, instructionsToken, input)
		}
		out[i] = arg
	}

	return out
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
