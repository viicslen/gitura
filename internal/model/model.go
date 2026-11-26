// Package model defines shared domain types for the gitura application.
// All types are used as DTOs between the Go backend and the Wails frontend.
package model

// PullRequestSummary is the top-level DTO for a loaded GitHub pull request.
type PullRequestSummary struct {
	ID              int64      `json:"id"`
	Number          int        `json:"number"`
	Title           string     `json:"title"`
	State           string     `json:"state"`
	IsDraft         bool       `json:"is_draft"`
	Body            string     `json:"body"`
	HeadBranch      string     `json:"head_branch"`
	HeadOwner       string     `json:"head_owner"`
	HeadRepo        string     `json:"head_repo"`
	BaseBranch      string     `json:"base_branch"`
	HeadSHA         string     `json:"head_sha"`
	NodeID          string     `json:"node_id"`
	HTMLURL         string     `json:"html_url"`
	AuthorLogin     string     `json:"author_login"`
	Owner           string     `json:"owner"`
	Repo            string     `json:"repo"`
	CommentCount    int        `json:"comment_count"`
	UnresolvedCount int        `json:"unresolved_count"`
	Labels          []LabelDTO `json:"labels"`
}

// LabelDTO is a PR label; Color is a 6-digit hex string without the leading "#".
type LabelDTO struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// ReviewCommentDTO represents a single review comment as a frontend-facing DTO.
// Author identity is flattened to string fields (no nested struct) to keep
// the Go↔JS serialization contract stable.
type ReviewCommentDTO struct {
	ID           int64  `json:"id"`
	InReplyToID  int64  `json:"in_reply_to_id"`
	ReviewID     int64  `json:"review_id"`
	Body         string `json:"body"`
	AuthorLogin  string `json:"author_login"`
	AuthorAvatar string `json:"author_avatar"`
	DiffHunk     string `json:"diff_hunk"`
	CreatedAt    string `json:"created_at"` // RFC3339
	IsSuggestion bool   `json:"is_suggestion"`
	HTMLURL      string `json:"html_url,omitempty"`
}

// CommentThreadDTO groups a root ReviewComment with all its replies.
// NodeID is the GraphQL global ID required for resolve/unresolve mutations.
// Resolved state is tracked at the thread level only; individual ReviewCommentDTOs
// do not carry a resolved flag.
type CommentThreadDTO struct {
	RootID   int64              `json:"root_id"`
	ReviewID int64              `json:"review_id"`
	NodeID   string             `json:"node_id"`
	Comments []ReviewCommentDTO `json:"comments"`
	Resolved bool               `json:"resolved"`
	Path     string             `json:"path"`
	Line     int                `json:"line"`
	Side     string             `json:"side"` // "LEFT" (old file) or "RIGHT" (new file)
	// Outdated is true when the code under the thread changed since it was posted.
	Outdated bool `json:"outdated"`
}

// IssueCommentDTO represents a general comment left on the pull request itself.
type IssueCommentDTO struct {
	ID           int64  `json:"id"`
	Body         string `json:"body"`
	AuthorLogin  string `json:"author_login"`
	AuthorAvatar string `json:"author_avatar"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	HTMLURL      string `json:"html_url"`
}

// ReviewDTO represents a submitted pull request review in the overview view.
type ReviewDTO struct {
	ReviewID            int64  `json:"review_id"`
	State               string `json:"state"`
	Body                string `json:"body"`
	AuthorLogin         string `json:"author_login"`
	AuthorAvatar        string `json:"author_avatar"`
	SubmittedAt         string `json:"submitted_at"`
	HTMLURL             string `json:"html_url"`
	AllThreadsResolved  bool   `json:"all_threads_resolved"`
	HasThreads          bool   `json:"has_threads"`
	ResolvedThreadCount int    `json:"resolved_thread_count"`
	TotalThreadCount    int    `json:"total_thread_count"`
}

// PROverviewDTO holds the data shown in the overview tab for a pull request.
type PROverviewDTO struct {
	Summary         PullRequestSummary `json:"summary"`
	GeneralComments []IssueCommentDTO  `json:"general_comments"`
	Reviews         []ReviewDTO        `json:"reviews"`
	// ReviewNumbers numbers every shown-worthy review 1..n in submission order, regardless of
	// filters, so "Review N" labels stay stable across tabs and toggles.
	ReviewNumbers map[int64]int `json:"review_numbers"`
}

// CommitDTO is a commit on a pull request's branch.
type CommitDTO struct {
	SHA          string `json:"sha"`
	Headline     string `json:"headline"`
	AuthorLogin  string `json:"author_login"`
	AuthorAvatar string `json:"author_avatar"`
	CommittedAt  string `json:"committed_at"` // RFC3339
	HTMLURL      string `json:"html_url"`
}

// PRTimelineDTO holds what the timeline tab interleaves with the review threads.
// Reviews holds every submitted review, reply-only ones included, so thread comments can be
// placed at the time their review was published rather than when they were drafted.
type PRTimelineDTO struct {
	Commits         []CommitDTO       `json:"commits"`
	Reviews         []ReviewDTO       `json:"reviews"`
	GeneralComments []IssueCommentDTO `json:"general_comments"`
	ReviewNumbers   map[int64]int     `json:"review_numbers"`
}

// AuthState represents the current authentication status of the app.
// Login and AvatarURL are flat string fields; the frontend never receives a
// nested User struct. Token is intentionally omitted from JSON serialization
// and is never sent to the frontend.
type AuthState struct {
	IsAuthenticated bool   `json:"is_authenticated"`
	Login           string `json:"login"`      // empty if not authenticated
	AvatarURL       string `json:"avatar_url"` // empty if not authenticated
}

// DeviceFlowInfo holds the data returned when starting a GitHub OAuth Device Flow.
type DeviceFlowInfo struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// PollResult represents the outcome of a single device-flow token poll.
// Status is one of: "pending", "complete", "expired", "error".
// Interval is set when GitHub returns slow_down — the frontend must reschedule
// polling to use this new interval (in seconds).
type PollResult struct {
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Interval int    `json:"interval,omitempty"` // non-zero only on slow_down
}

// IgnoredCommenterDTO represents a persisted ignored-commenter entry.
type IgnoredCommenterDTO struct {
	Login string `json:"login" toml:"login"`
}

// SuggestionCommitResult holds the outcome of committing a suggestion.
type SuggestionCommitResult struct {
	CommitSHA string `json:"commit_sha"`
	HTMLURL   string `json:"html_url"`
}

// PRListItem is a lightweight DTO for a single row in the PR list view.
// It carries only the fields required by FR-002 plus navigation data.
// IsAuthor, IsAssignee, IsReviewer are derived from the GitHub Search result
// and are used by the frontend for client-side involvement toggle filtering.
type PRListItem struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Owner       string `json:"owner"` // GitHub org or user owning the repo
	Repo        string `json:"repo"`  // repository name (without owner prefix)
	AuthorLogin string `json:"author_login"`
	CreatedAt   string `json:"created_at"` // RFC3339
	UpdatedAt   string `json:"updated_at"` // RFC3339 — used for default sort
	HTMLURL     string `json:"html_url"`   // canonical GitHub PR URL
	IsDraft     bool   `json:"is_draft"`
	State       string `json:"state"`               // "open", "closed" or "merged"
	IsAuthor    bool   `json:"is_author"`           // true when login == PR author
	IsAssignee  bool   `json:"is_assignee"`         // true when login is in assignees
	IsReviewer  bool   `json:"is_reviewer"`         // true when login is in requested_reviewers
	NodeID      string `json:"-"`                   // GraphQL ID, used to look up the branch fields below
	BaseRef     string `json:"base_ref,omitempty"`  // branch the PR merges into; empty when not looked up
	HeadRef     string `json:"head_ref,omitempty"`  // branch the PR merges from
	HeadRepo    string `json:"head_repo,omitempty"` // "owner/repo" holding HeadRef; empty when that repository is gone
}

// PRListFilters carries the active filter state from the frontend to the
// Go backend. Only IncludeDrafts affects the server-side query; all other
// fields are used for client-side filtering in the frontend.
// At least one of IncludeAuthor, IncludeAssignee, IncludeReviewer must be true.
type PRListFilters struct {
	IncludeAuthor   bool   `json:"include_author"`   // client-side: show PRs authored by the user
	IncludeAssignee bool   `json:"include_assignee"` // client-side: show PRs assigned to the user
	IncludeReviewer bool   `json:"include_reviewer"` // client-side: show PRs where user is review-requested
	Repo            string `json:"repo"`             // client-side: "owner/repo" format, or "" for all
	Org             string `json:"org"`              // client-side: GitHub org login, or "" for all
	Author          string `json:"author"`           // client-side: filter by PR author login, or ""
	UpdatedAfter    string `json:"updated_after"`    // client-side: RFC3339 datetime, or "" for no date filter
	IncludeDrafts   bool   `json:"include_drafts"`   // server-side: false = exclude drafts (default)
}

// PRListResult is the return type of ListOpenPRs. On success, Items is populated
// and Error is empty. On error, Items may be empty and Error describes the failure.
// RateLimitReset is set (RFC3339) when the GitHub API rate limit was exhausted.
// IncompleteResults is true when GitHub returned incomplete_results=true.
// Items always contains all matching PRs — client-side filtering is applied
// in the frontend using the involvement and text filter fields.
type PRListResult struct {
	Items             []PRListItem `json:"items"`
	RateLimitReset    string       `json:"rate_limit_reset,omitempty"` // RFC3339
	IncompleteResults bool         `json:"incomplete_results,omitempty"`
	Error             string       `json:"error,omitempty"`
}

// PRFileDTO represents a single file changed in a pull request.
// Returned by GetPRFiles; does not include diff content.
type PRFileDTO struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"` // "added"|"removed"|"modified"|"renamed"|"copied"
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	PreviousFilename string `json:"previous_filename,omitempty"` // non-empty for renames
	IsBinary         bool   `json:"is_binary"`
	Viewed           bool   `json:"viewed"` // marked as viewed by the user on GitHub
	// ViewedLocal means the Viewed mark of this submodule file is kept in gitura, not on a GitHub PR.
	ViewedLocal bool `json:"viewed_local,omitempty"`
	// LinesUnknown means Additions/Deletions are not counted yet: the file's content loads when it is opened.
	LinesUnknown bool `json:"lines_unknown,omitempty"`
	// Repo is the repository holding the row, which with Filename identifies it: "" for the loaded change,
	// else a submodule's path. A pointer row's is the repository holding that submodule.
	Repo string `json:"repo,omitempty"`
	// Submodule is the path of the submodule a file belongs to. The submodule's own row
	// (Filename == Submodule, Status "submodule") carries the fields below.
	Submodule          string `json:"submodule,omitempty"`
	SubmoduleRange     string `json:"submodule_range,omitempty"`     // "old..new" short SHAs; new may end in "-dirty"
	SubmoduleBackwards bool   `json:"submodule_backwards,omitempty"` // the bump rewinds to an ancestor
	SubmoduleDiverged  bool   `json:"submodule_diverged,omitempty"`  // neither commit is an ancestor of the other
	// SubmodulePR names the submodule's open PR whose head is the bumped commit ("owner/repo#12");
	// review comments and viewed marks on its files go there.
	SubmodulePR    string `json:"submodule_pr,omitempty"`
	SubmodulePRURL string `json:"submodule_pr_url,omitempty"`
	// SubmodulePRUnknown means looking up the submodule's open PR failed, as opposed to finding none.
	SubmodulePRUnknown bool `json:"submodule_pr_unknown,omitempty"`
	// SubmoduleViewedUnknown means reading the Viewed marks on the submodule's PR failed; its files may show unviewed wrongly.
	SubmoduleViewedUnknown bool `json:"submodule_viewed_unknown,omitempty"`
	// SubmoduleUnavailable says why the submodule's files are missing, e.g. "no access".
	SubmoduleUnavailable string `json:"submodule_unavailable,omitempty"`
}

// FileKeyDTO identifies a file in the Files list: one path can be both a file of the loaded change
// and a file inside a submodule, e.g. when a tracked directory becomes a submodule.
type FileKeyDTO struct {
	// Submodule is the file's PRFileDTO.Repo.
	Submodule string `json:"submodule,omitempty"`
	Path      string `json:"path"`
	// Pointer marks a submodule's pointer row (Status "submodule"), apart from a regular file at its path.
	Pointer bool `json:"pointer,omitempty"`
}

// ViewedFilesDTO is what GetViewedFiles reports.
type ViewedFilesDTO struct {
	Files []FileKeyDTO `json:"files"`
	// UnknownSubmodules are the submodules whose PR's Viewed marks couldn't be read; see PRFileDTO.SubmoduleViewedUnknown.
	UnknownSubmodules []string `json:"unknown_submodules,omitempty"`
}

// DiffLineType classifies a single line in a diff hunk.
type DiffLineType string

const (
	// DiffLineContext is a line present in both old and new versions (no change).
	DiffLineContext DiffLineType = "context"
	// DiffLineAdd is a line added in the new version.
	DiffLineAdd DiffLineType = "add"
	// DiffLineDelete is a line removed from the old version.
	DiffLineDelete DiffLineType = "delete"
)

// DiffLineDTO represents a single parsed line within a diff hunk.
// OldNo and NewNo are 0 when the line does not exist on that side.
type DiffLineDTO struct {
	Type    DiffLineType `json:"type"`
	OldNo   int          `json:"old_no"`  // line number in old file; 0 for add lines
	NewNo   int          `json:"new_no"`  // line number in new file; 0 for delete lines
	Content string       `json:"content"` // line text without diff prefix character
}

// DiffHunkDTO represents a contiguous block of changed lines within a file diff.
type DiffHunkDTO struct {
	Header   string        `json:"header"` // raw hunk header, e.g. "@@ -10,7 +12,9 @@"
	OldStart int           `json:"old_start"`
	OldLines int           `json:"old_lines"`
	NewStart int           `json:"new_start"`
	NewLines int           `json:"new_lines"`
	Lines    []DiffLineDTO `json:"lines"`
}

// ParsedDiffDTO is the structured representation of a single file's diff.
// Returned by GetFileDiff. The frontend renders this directly without further parsing.
type ParsedDiffDTO struct {
	Filename         string        `json:"filename"`
	PreviousFilename string        `json:"previous_filename,omitempty"`
	Status           string        `json:"status"`
	IsBinary         bool          `json:"is_binary"`
	TotalAdditions   int           `json:"total_additions"`
	TotalDeletions   int           `json:"total_deletions"`
	Hunks            []DiffHunkDTO `json:"hunks"`
}

// DraftCommentDTO is the request payload for creating an inline review comment.
// Uses comfort-fade positioning (line + side), not legacy diff position integers.
// StartLine and StartSide are only required for multi-line comments.
// Line 0 marks a comment on the whole file rather than on its lines.
type DraftCommentDTO struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line"`                 // end line (or only line for single-line)
	Side      string `json:"side"`                 // "LEFT" or "RIGHT"
	StartLine int    `json:"start_line,omitempty"` // 0 means single-line comment
	StartSide string `json:"start_side,omitempty"` // "LEFT" or "RIGHT"; required when StartLine > 0
	// Submodule is the PRFileDTO.Repo of the file commented on. It stays set if the bump later disappears,
	// so the draft is folded into the review body.
	Submodule string `json:"submodule,omitempty"`
	// Pointer is set on a comment on a submodule's pointer row, as FileKeyDTO.Pointer.
	Pointer bool `json:"pointer,omitempty"`
	// HeadSHA is the PR head the comment was written against; a mismatch means the PR changed meanwhile.
	HeadSHA string `json:"head_sha,omitempty"`
	// Range is the submodule's "old..new" (SubmoduleRange) the comment was written against; a mismatch means the bump changed meanwhile.
	Range string `json:"range,omitempty"`
	// ID identifies the pending draft through edits; the backend assigns it.
	ID int64 `json:"-"`
	// Bump is the submodule's "old..new" commits when the draft was added; the backend fills it.
	Bump string `json:"-"`
	// DiffHunk is the hunk GitHub shows a restored draft under, ending at its line.
	DiffHunk string `json:"-"`
}

// PendingReviewDTO describes the current pending review state.
// ReviewID of 0 means no pending review has been created on GitHub yet.
type PendingReviewDTO struct {
	ReviewID   int64             `json:"review_id"` // 0 = none
	Comments   []DraftCommentDTO `json:"comments"`
	HasPending bool              `json:"has_pending"`
	HeadSHA    string            `json:"head_sha"` // commit the comments' line numbers refer to
	// InFlightReview is the review an earlier submit whose outcome is unknown sent; SubmitReview refuses another
	// verdict or text until a submit settles it.
	InFlightReview *InFlightReviewDTO `json:"in_flight_review,omitempty"`
}

// InFlightReviewDTO is the verdict and text of a review sent by a submit whose outcome is unknown.
type InFlightReviewDTO struct {
	Verdict string `json:"verdict"`
	Body    string `json:"body"`
}

// ReviewSubmitRequest is the request payload for submitting a review verdict.
type ReviewSubmitRequest struct {
	Body    string `json:"body,omitempty"` // optional top-level review message
	Verdict string `json:"verdict"`        // "APPROVE" | "REQUEST_CHANGES" | "COMMENT"
	// Submodules says what to do with the drafts that can go to each submodule's PR, one per submodule
	// with drafts; a "fold" choice has them added to Body.
	Submodules []SubmoduleReviewChoice `json:"submodules,omitempty"`
}

// SubmoduleReviewChoice is what SubmitReview does with the drafts on one submodule's files that
// fall inside its PR's diff; the others are always added to the review body.
type SubmoduleReviewChoice struct {
	Path    string `json:"path"`              // the submodule's path in the loaded PR
	Mode    string `json:"mode"`              // "review" — a review on the submodule PR | "comments" — single comments there | "fold" — into the review body
	Verdict string `json:"verdict,omitempty"` // "review" mode: "APPROVE" | "REQUEST_CHANGES" | "COMMENT"
	Body    string `json:"body,omitempty"`    // "review" mode: the review's text
	// PR, ToPR and Folded are the routing the dialog showed for the submodule, as SubmoduleDrafts reported it;
	// every submodule with drafts needs a choice, "fold" for one shown without any.
	PR     string `json:"pr,omitempty"`
	ToPR   int    `json:"to_pr"`
	Folded int    `json:"folded"`
}

// SubmoduleDraftsDTO says where SubmitReview would send the pending drafts on one submodule's files.
type SubmoduleDraftsDTO struct {
	Path      string `json:"path"`
	PR        string `json:"pr,omitempty"` // the submodule's open PR, "owner/repo#12"
	PRURL     string `json:"pr_url,omitempty"`
	ToPR      int    `json:"to_pr"`                // drafts inside that PR's diff, which a SubmoduleReviewChoice routes
	Folded    int    `json:"folded"`               // drafts added to the review body whatever the choice
	OwnPR     bool   `json:"own_pr,omitempty"`     // the viewer wrote that PR, so GitHub refuses Approve and Request changes
	PRUnknown bool   `json:"pr_unknown,omitempty"` // the open-PR lookup failed; its drafts fold unless a retry finds the PR
	// InFlight is the mode, "review" or "comments", an earlier submit whose outcome is unknown sent drafts to
	// the PR with; SubmitReview refuses another until a submit settles it.
	InFlight string `json:"in_flight,omitempty"`
	// InFlightReview is what that submit sent in "review" mode, which the next one must send too.
	InFlightReview *InFlightReviewDTO `json:"in_flight_review,omitempty"`
	// Outdated counts the drafts written against another bump of the submodule than the loaded one, which fold.
	Outdated int `json:"outdated,omitempty"`
	// Rerouted says looking the PR up again changed which files it takes viewed marks on, or their marks.
	Rerouted bool `json:"rerouted,omitempty"`
}

// ReviewSubmitResult is returned after a successful review submission.
type ReviewSubmitResult struct {
	ReviewID int64  `json:"review_id"`
	HTMLURL  string `json:"html_url"`
	// SubmoduleReviewURLs links the reviews posted on submodule PRs for comments on submodule files.
	SubmoduleReviewURLs []string `json:"submodule_review_urls,omitempty"`
	// SubmoduleCommentURLs links the single comments posted on submodule PRs instead of a review.
	SubmoduleCommentURLs []string `json:"submodule_comment_urls,omitempty"`
	// FoldedComments counts submodule-file comments written into the review body instead.
	FoldedComments int `json:"folded_comments,omitempty"`
}

// CommandDTO is a named CLI invocation template configured by the user.
// Command may contain the literal string "{{instructions}}" as a placeholder;
// if present, the input text is substituted as a shell argument rather than
// piped via stdin. It may also use "{{repo_path}}" and the CommandVariableDTO
// placeholders.
type CommandDTO struct {
	Name    string `json:"name" toml:"name"`
	Command string `json:"command" toml:"command"`
}

// CommandVariableDTO is a placeholder a command template can use as "{{Name}}".
type CommandVariableDTO struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AppearanceDTO holds the user's font and text-size choices. Values are option
// keys the frontend understands; empty means its default.
type AppearanceDTO struct {
	TextFont string `json:"text_font" toml:"text_font"`
	CodeFont string `json:"code_font" toml:"code_font"`
	TextSize string `json:"text_size" toml:"text_size"`
	// Theme is "auto" (follow the system), "light" or "dark".
	Theme string `json:"theme" toml:"theme"`
	// Effects is "on" or "off" (the default) for the showpiece motion (springs, light, confetti).
	Effects string `json:"effects" toml:"effects"`
}

// RunContext carries optional provenance metadata attached to a command run.
// ThreadRootID identifies the review thread from which the run was launched;
// 0 means the run was not launched from a specific thread.
// CommentID identifies the specific comment within the thread; 0 means
// the run was launched from the thread body or a non-comment context.
type RunContext struct {
	ThreadRootID int64 `json:"thread_root_id,omitempty"`
	CommentID    int64 `json:"comment_id,omitempty"`
}

// RunOutput is a chunk of a running command's output; Stream is "stdout" or "stderr".
type RunOutput struct {
	RunID  string `json:"run_id"`
	Stream string `json:"stream"`
	Chunk  string `json:"chunk"`
}

// LoadProgress reports how many review threads of Total have loaded; Total is -1 while unknown.
type LoadProgress struct {
	Loaded int `json:"loaded"`
	Total  int `json:"total"`
}

// RunResult holds the outcome of a single command execution.
// Running is true while the goroutine has not yet completed; the frontend
// should treat it as a live/pending entry until a completion event arrives.
// Cancelled is true when the run was stopped by the user before it finished.
// ThreadRootID and CommentID are copied from the RunContext at launch time.
type RunResult struct {
	RunID        string `json:"run_id"`
	CommandName  string `json:"command_name"`
	Input        string `json:"input"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exit_code"`
	StartedAt    string `json:"started_at"`  // RFC3339
	FinishedAt   string `json:"finished_at"` // RFC3339; empty while running
	Running      bool   `json:"running"`
	Cancelled    bool   `json:"cancelled"`
	ThreadRootID int64  `json:"thread_root_id,omitempty"`
	CommentID    int64  `json:"comment_id,omitempty"`
}
