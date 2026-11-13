package app

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitura/internal/ghapi"
	"gitura/internal/gitlocal"
	"gitura/internal/logger"
	"gitura/internal/model"
)

type launchTarget struct {
	dir, repo, ref string
	local          bool
}

// Launch is what the command line asked to open: a PR, or local changes at LocalPath.
type Launch struct {
	PR        *model.PullRequestSummary `json:"pr,omitempty"`
	LocalPath string                    `json:"local_path,omitempty"`
}

type launchArgs struct {
	target           *launchTarget
	newWindow, focus bool
}

// parseArgs reads `[--new-window | --focus] [--repo owner/repo] [PR number | branch]` or
// `[--new-window | --focus] --local [path]`; dir is the directory whose origin remote names the
// repository when --repo is absent, and the default --local path.
func parseArgs(args []string, dir string, usage io.Writer) (launchArgs, error) {
	fs := flag.NewFlagSet("gitura", flag.ContinueOnError)
	fs.SetOutput(usage)
	repo := fs.String("repo", "", "`owner/repo` of the PR (default: the origin remote of the current directory)")
	newWindow := fs.Bool("new-window", false, "open a separate window instead of reusing the running one")
	focus := fs.Bool("focus", false, "open the PR or local changes in a separate window that shows only that review")
	local := fs.Bool("local", false, "review the local changes of the repository at `path` (default: the current directory)")
	fs.Usage = func() {
		if _, err := fmt.Fprintln(fs.Output(), "Usage: gitura [--new-window | --focus] [--repo owner/repo] [PR number | branch]\n       gitura [--new-window | --focus] --local [path]"); err == nil {
			fs.PrintDefaults()
		}
	}
	if err := fs.Parse(args); err != nil {
		return launchArgs{}, err
	}
	la := launchArgs{newWindow: *newWindow || *focus, focus: *focus}
	if *local {
		if *repo != "" {
			err := fmt.Errorf("--repo and --local can't be combined")
			_, _ = fmt.Fprintln(fs.Output(), err)
			return launchArgs{}, err
		}
		la.target = &launchTarget{dir: dir, ref: fs.Arg(0), local: true}
	} else if ref := fs.Arg(0); ref != "" {
		la.target = &launchTarget{dir: dir, repo: *repo, ref: ref}
	}
	if la.focus && la.target == nil {
		err := fmt.Errorf("--focus needs a PR or --local")
		_, _ = fmt.Fprintln(fs.Output(), err)
		return launchArgs{}, err
	}
	return la, nil
}

// ConfigureLaunch applies the command line: it queues the PR or local changes to open and, unless
// --new-window or --focus is given, makes later launches hand their PR to this window and exit.
// It returns flag.ErrHelp for -h. It is a function, not a method, so Wails doesn't bind it.
func ConfigureLaunch(opts *application.Options, a *App, args []string, dir string, usage io.Writer) error {
	la, err := parseArgs(args, dir, usage)
	if err != nil {
		return err
	}
	a.launch.Store(la.target)
	a.focus = la.focus
	if la.newWindow {
		return nil
	}
	opts.SingleInstance = &application.SingleInstanceOptions{
		UniqueID:               "gitura",
		OnSecondInstanceLaunch: a.secondInstance,
	}
	return nil
}

// secondInstance runs when another launch handed over its command line; that process
// already validated it, so a parse error here is only logged.
func (a *App) secondInstance(data application.SecondInstanceData) {
	if w, ok := mainWindow(); ok {
		w.UnMinimise()
		w.Show()
		w.Focus()
	}
	args := data.Args
	if len(args) > 0 {
		args = args[1:] // Wails v3 passes the whole os.Args, executable included
	}
	la, err := parseArgs(args, data.WorkingDir, io.Discard)
	if err != nil {
		logger.L.Warn("second instance: bad arguments", "args", data.Args, "err", err)
		return
	}
	if la.target != nil {
		a.launch.Store(la.target)
		a.emit("launch")
	}
}

// IsFocused reports whether this window was opened with --focus, so it shows only the launched review.
func (a *App) IsFocused() bool {
	return a.focus
}

// TakeLaunch resolves what the command line asked to open; it returns nil when there is
// nothing or it was already taken. "launch" is emitted when a later launch queues one.
//
// Error prefixes: "auth:", "validation:", "git:", "notfound:", "github:".
func (a *App) TakeLaunch() (*Launch, error) {
	if a.ghClient == nil {
		return nil, fmt.Errorf("auth: not authenticated")
	}
	t := a.launch.Swap(nil)
	if t == nil {
		return nil, nil
	}
	if t.local {
		path := t.ref
		if !filepath.IsAbs(path) {
			path = filepath.Join(t.dir, path)
		}
		dir, err := gitlocal.TopLevel(a.ctx, path)
		if err != nil {
			return nil, err
		}
		if pr := a.openPRForPushedBranch(dir); pr != nil {
			return &Launch{PR: pr}, nil
		}
		return &Launch{LocalPath: dir}, nil
	}
	pr, err := a.resolveLaunchPR(t)
	if err != nil {
		return nil, err
	}
	return &Launch{PR: pr}, nil
}

// openPRForPushedBranch returns the open PR for dir's branch when everything local is
// committed and pushed, so there is nothing to review beyond the PR; nil otherwise.
func (a *App) openPRForPushedBranch(dir string) *model.PullRequestSummary {
	branch, ok := gitlocal.PushedBranch(a.ctx, dir)
	if !ok {
		return nil
	}
	pr, err := a.resolveLaunchPR(&launchTarget{dir: dir, ref: branch})
	if err != nil {
		logger.L.Debug("local launch: no PR for pushed branch", "branch", branch, "err", err)
		return nil
	}
	if pr.State != "open" {
		return nil
	}
	return pr
}

func (a *App) resolveLaunchPR(t *launchTarget) (*model.PullRequestSummary, error) {
	owner, repo, ok := strings.Cut(t.repo, "/")
	if t.repo == "" {
		var err error
		if owner, repo, err = gitlocal.OriginRepo(a.ctx, t.dir); err != nil {
			return nil, err
		}
	} else if !ok || owner == "" || repo == "" {
		return nil, fmt.Errorf("validation: --repo must be owner/repo, got %q", t.repo)
	}

	number, err := strconv.Atoi(strings.TrimPrefix(t.ref, "#"))
	if err != nil {
		if number, err = ghapi.FindPRByBranch(a.ctx, a.ghClient, owner, repo, t.ref); err != nil {
			return nil, err
		}
	}
	return ghapi.FetchPR(a.ctx, a.ghClient, owner, repo, number)
}
