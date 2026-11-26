package app

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"gitura/internal/logger"
	"gitura/internal/model"
	"gitura/internal/runner"
)

// RunCommands launches one or more commands in parallel against
// the same input text. For each command a pending RunResult with
// Running=true is emitted immediately via the "command:run:pending" event.
// While each process runs, raw stdout/stderr chunks are emitted via the
// "command:run:output" event. When each process finishes, the completed
// result is emitted via the "command:run:complete" event.
//
// Returns the pending RunResult stubs (one per command) so the frontend can
// seed its run list synchronously before events arrive.
//
// Error prefix: "validation:" — empty command names; "settings:" — load failure.
func (a *App) RunCommands(commandNames []string, input string, runCtx model.RunContext) ([]model.RunResult, error) {
	if len(commandNames) == 0 {
		return nil, fmt.Errorf("validation: at least one command name required")
	}
	if err := a.loadConfig(); err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}

	localPath := ""
	if l := a.local.Load(); l != nil {
		localPath = l.dir
	} else if a.prNumber > 0 {
		localPath = a.prLocalPath(a.prOwner, a.prRepo, a.prNumber)
	}

	vars := a.runVars(runCtx)

	// Build a lookup map for fast name -> command resolution.
	commands := a.conf().Commands
	cmdMap := make(map[string]model.CommandDTO, len(commands))
	for _, c := range commands {
		cmdMap[c.Name] = c
	}

	pendingResults := make([]model.RunResult, 0, len(commandNames))
	now := time.Now().UTC().Format(time.RFC3339)

	var wg sync.WaitGroup
	for _, name := range commandNames {
		cmd, ok := cmdMap[name]
		if !ok {
			logger.L.Warn("RunCommands: unknown command name", "name", name)
			continue
		}

		runID := uuid.NewString()

		// Create a cancellable context for this run and register the cancel func.
		cancelCtx, cancel := context.WithCancel(context.Background())
		a.runCancelsMu.Lock()
		a.runCancels[runID] = cancel
		a.runCancelsMu.Unlock()

		// Emit pending immediately so the frontend can show a spinner.
		pending := model.RunResult{
			RunID:        runID,
			CommandName:  cmd.Name,
			Input:        input,
			StartedAt:    now,
			Running:      true,
			ThreadRootID: runCtx.ThreadRootID,
			CommentID:    runCtx.CommentID,
		}
		pendingResults = append(pendingResults, pending)
		a.emit("command:run:pending", pending)

		wg.Add(1)
		go func(c model.CommandDTO, rID string, ctx context.Context, lp string, rc model.RunContext) {
			defer wg.Done()
			defer func() {
				a.runCancelsMu.Lock()
				delete(a.runCancels, rID)
				a.runCancelsMu.Unlock()
			}()
			result := runner.Run(ctx, c, input, lp, vars, func(stream string, chunk string) {
				a.emit("command:run:output", model.RunOutput{RunID: rID, Stream: stream, Chunk: chunk})
			})
			result.RunID = rID
			result.Running = false
			result.ThreadRootID = rc.ThreadRootID
			result.CommentID = rc.CommentID
			a.emit("command:run:complete", result)
			logger.L.Info("run complete",
				"run_id", rID,
				"command", c.Name,
				"exit_code", result.ExitCode,
				"cancelled", result.Cancelled,
			)
		}(cmd, runID, cancelCtx, localPath, runCtx)
	}

	return pendingResults, nil
}

// runVars fills every placeholder, unknown ones with "", so none reaches the command as a literal "{{...}}".
func (a *App) runVars(runCtx model.RunContext) map[string]string {
	vars := make(map[string]string, len(runner.Variables))
	for _, v := range runner.Variables {
		vars[v.Name] = ""
	}
	if pr := a.prCache; pr != nil {
		vars["pr_number"] = strconv.Itoa(pr.Number)
		vars["pr_title"] = pr.Title
		vars["pr_url"] = pr.HTMLURL
		vars["repo"] = pr.Owner + "/" + pr.Repo
		vars["head_branch"] = pr.HeadBranch
		vars["base_branch"] = pr.BaseBranch
		vars["author"] = pr.AuthorLogin
	}
	for _, t := range a.threads {
		if t.RootID != runCtx.ThreadRootID || runCtx.ThreadRootID == 0 {
			continue
		}
		vars["file"] = t.Path
		if t.Line > 0 {
			vars["line"] = strconv.Itoa(t.Line)
		}
		for _, c := range t.Comments {
			if c.ID == runCtx.CommentID {
				vars["comment_author"] = c.AuthorLogin
			}
		}
	}

	return vars
}

// CancelRun stops an in-flight command by its run ID.
// If the run has already finished or the ID is unknown, this is a no-op.
//
// Error prefix: "not_found:" — unknown or already-finished run ID.
func (a *App) CancelRun(runID string) error {
	a.runCancelsMu.Lock()
	cancel, ok := a.runCancels[runID]
	a.runCancelsMu.Unlock()
	if !ok {
		return fmt.Errorf("not_found: run %q is not in-flight", runID)
	}
	cancel()
	return nil
}
