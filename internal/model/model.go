// Package model defines shared domain types for the gitura application.
// All types are used as DTOs between the Go backend and the Wails frontend.
package model

import "time"

// User represents a GitHub user (commenter or authenticated user).
type User struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

// PullRequestSummary is the top-level DTO for a loaded GitHub pull request.
type PullRequestSummary struct {
	ID              int64  `json:"id"`
	Number          int    `json:"number"`
	Title           string `json:"title"`
	State           string `json:"state"`
	IsDraft         bool   `json:"is_draft"`
	Body            string `json:"body"`
	HeadBranch      string `json:"head_branch"`
	BaseBranch      string `json:"base_branch"`
	HeadSHA         string `json:"head_sha"`
	NodeID          string `json:"node_id"`
	HTMLURL         string `json:"html_url"`
	Owner           string `json:"owner"`
	Repo            string `json:"repo"`
	CommentCount    int    `json:"comment_count"`
	UnresolvedCount int    `json:"unresolved_count"`
}

// CommentDTO represents a single review comment as a frontend-facing DTO.
// Author identity is flattened to string fields (no nested struct) to keep
// the Go↔JS serialization contract stable.
type CommentDTO struct {
	ID           int64  `json:"id"`
	InReplyToID  int64  `json:"in_reply_to_id"`
	Body         string `json:"body"`
	AuthorLogin  string `json:"author_login"`
	AuthorAvatar string `json:"author_avatar"`
	DiffHunk     string `json:"diff_hunk"`
	CreatedAt    string `json:"created_at"` // RFC3339
	IsSuggestion bool   `json:"is_suggestion"`
}

// CommentThreadDTO groups a root ReviewComment with all its replies.
// NodeID is the GraphQL global ID required for resolve/unresolve mutations.
// Resolved state is tracked at the thread level only; individual CommentDTOs
// do not carry a resolved flag.
type CommentThreadDTO struct {
	RootID   int64        `json:"root_id"`
	NodeID   string       `json:"node_id"`
	Comments []CommentDTO `json:"comments"`
	Resolved bool         `json:"resolved"`
	Path     string       `json:"path"`
	Line     int          `json:"line"`
	// Outdated is true when GitHub reports the thread's current line as null
	// (the surrounding code changed since the comment was posted).
	Outdated bool `json:"outdated"`
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
	Login   string    `json:"login" toml:"login"`
	AddedAt time.Time `json:"added_at" toml:"added_at"`
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
	IsAuthor    bool   `json:"is_author"`   // true when login == PR author
	IsAssignee  bool   `json:"is_assignee"` // true when login is in assignees
	IsReviewer  bool   `json:"is_reviewer"` // true when login is in requested_reviewers
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
type DraftCommentDTO struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line"`                 // end line (or only line for single-line)
	Side      string `json:"side"`                 // "LEFT" or "RIGHT"
	StartLine int    `json:"start_line,omitempty"` // 0 means single-line comment
	StartSide string `json:"start_side,omitempty"` // "LEFT" or "RIGHT"; required when StartLine > 0
}

// PendingReviewDTO describes the current pending review state.
// ReviewID of 0 means no pending review has been created on GitHub yet.
type PendingReviewDTO struct {
	ReviewID   int64             `json:"review_id"` // 0 = none
	Comments   []DraftCommentDTO `json:"comments"`
	HasPending bool              `json:"has_pending"`
}

// ReviewSubmitDTO is the request payload for submitting a review verdict.
type ReviewSubmitDTO struct {
	Body    string `json:"body,omitempty"` // optional top-level review message
	Verdict string `json:"verdict"`        // "APPROVE" | "REQUEST_CHANGES" | "COMMENT"
}

// ReviewSubmitResult is returned after a successful review submission.
type ReviewSubmitResult struct {
	ReviewID int64  `json:"review_id"`
	HTMLURL  string `json:"html_url"`
}

// CommandDTO is a named CLI invocation template configured by the user.
// Command may contain the literal string "{{instructions}}" as a placeholder;
// if present, the input text is substituted as a shell argument rather than
// piped via stdin.
type CommandDTO struct {
	ID      string `json:"id" toml:"id"`
	Name    string `json:"name" toml:"name"`
	Command string `json:"command" toml:"command"`
}

// RunResult holds the outcome of a single command execution.
// Running is true while the goroutine has not yet completed; the frontend
// should treat it as a live/pending entry until a completion event arrives.
// Cancelled is true when the run was stopped by the user before it finished.
type RunResult struct {
	RunID       string `json:"run_id"`
	CommandID   string `json:"command_id"`
	CommandName string `json:"command_name"`
	Input       string `json:"input"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exit_code"`
	StartedAt   string `json:"started_at"`  // RFC3339
	FinishedAt  string `json:"finished_at"` // RFC3339; empty while running
	Running     bool   `json:"running"`
	Cancelled   bool   `json:"cancelled"`
}
