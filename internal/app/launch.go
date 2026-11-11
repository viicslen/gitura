package app

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

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
	target    *launchTarget
	newWindow bool
}

// parseArgs reads `[--new-window] [--repo owner/repo] [PR number | branch]` or
// `[--new-window] --local [path]`; dir is the directory whose origin remote names the
// repository when --repo is absent, and the default --local path.
func parseArgs(args []string, dir string, usage io.Writer) (launchArgs, error) {
	fs := flag.NewFlagSet("gitura", flag.ContinueOnError)
	fs.SetOutput(usage)
	repo := fs.String("repo", "", "`owner/repo` of the PR (default: the origin remote of the current directory)")
	newWindow := fs.Bool("new-window", false, "open a separate window instead of reusing the running one")
	local := fs.Bool("local", false, "review the local changes of the repository at `path` (default: the current directory)")
	fs.Usage = func() {
		if _, err := fmt.Fprintln(fs.Output(), "Usage: gitura [--new-window] [--repo owner/repo] [PR number | branch]\n       gitura [--new-window] --local [path]"); err == nil {
			fs.PrintDefaults()
		}
	}
	if err := fs.Parse(args); err != nil {
		return launchArgs{}, err
	}
	la := launchArgs{newWindow: *newWindow}
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
	return la, nil
}

// ConfigureLaunch applies the command line: it queues the PR or local changes to open and, unless
// --new-window is given, makes later launches hand their PR to this window and exit.
// It returns flag.ErrHelp for -h. It is a function, not a method, so Wails doesn't bind it.
func ConfigureLaunch(opts *options.App, a *App, args []string, dir string, usage io.Writer) error {
	la, err := parseArgs(args, dir, usage)
	if err != nil {
		return err
	}
	a.launch.Store(la.target)
	if la.newWindow {
		return nil
	}
	opts.SingleInstanceLock = &options.SingleInstanceLock{
		UniqueId:               "gitura",
		OnSecondInstanceLaunch: a.secondInstance,
	}
	return nil
}

// secondInstance runs when another launch handed over its command line; that process
// already validated it, so a parse error here is only logged.
func (a *App) secondInstance(data options.SecondInstanceData) {
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	la, err := parseArgs(data.Args, data.WorkingDirectory, io.Discard)
	if err != nil {
		logger.L.Warn("second instance: bad arguments", "args", data.Args, "err", err)
		return
	}
	if la.target != nil {
		a.launch.Store(la.target)
		a.emit("launch")
	}
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
		return &Launch{LocalPath: dir}, nil
	}
	pr, err := a.resolveLaunchPR(t)
	if err != nil {
		return nil, err
	}
	return &Launch{PR: pr}, nil
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
