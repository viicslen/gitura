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
	Login   string    `json:"login"`
	AddedAt time.Time `json:"added_at"`
}

// SuggestionCommitResult holds the outcome of committing a suggestion.
type SuggestionCommitResult struct {
	CommitSHA string `json:"commit_sha"`
	HTMLURL   string `json:"html_url"`
}
