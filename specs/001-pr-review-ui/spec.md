# Feature Specification: PR Review UI

**Feature Branch**: `001-pr-review-ui`
**Created**: 2026-03-30
**Status**: Draft
**Input**: User description: "Build a graphical application that can help me review PRs
(Review/PR comments, code changes, etc). PR comments can be displayed in a summary list
view or on a one-by-one basis, they can be replied to, marked as resolved, or suggested
changes commited to the branch. I want to be able to set ignored commenters for CI comments."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse and Triage PR Comments (Priority: P1)

A reviewer opens a pull request in the app and sees all review comments in a summary
list. Each comment shows the author, file, line reference, and a snippet of the comment
body. The reviewer can scan the full list to understand the review landscape before
diving into individual comments.

**Why this priority**: The list view is the entry point to all other comment
interactions. Without it, no other comment workflow is accessible.

**Independent Test**: Open any PR with multiple review comments and verify a summary
list renders all comments with author, location, and body preview.

**Acceptance Scenarios**:

1. **Given** a PR with 10 review comments from 3 different reviewers,
   **When** the user opens the PR in the app,
   **Then** a summary list displays all 10 comments with author name, file path,
   and a text excerpt for each.
2. **Given** a PR with review comments where some authors are on the ignored list,
   **When** the list is displayed,
   **Then** ignored-commenter comments are hidden and not counted in the summary.
3. **Given** the summary list, **When** the user selects a specific comment,
   **Then** the app transitions to the one-by-one detail view starting at that comment.

---

### User Story 2 - Navigate Comments One-by-One (Priority: P2)

The reviewer steps through each review comment individually, seeing full comment text,
the surrounding code diff context, and any existing replies in the thread. Navigation
controls move forward and backward through the comment queue.

**Why this priority**: Linear navigation lets the reviewer process comments
methodically, reducing the chance of missing any feedback.

**Independent Test**: Open a PR with at least 3 comments, use next/previous controls
to navigate all of them, and confirm each shows full text and diff context.

**Acceptance Scenarios**:

1. **Given** the one-by-one view on a comment, **When** the user presses "Next",
   **Then** the next unresolved comment in sequence is displayed with its full text
   and surrounding diff context.
2. **Given** the one-by-one view on the last comment, **When** the user presses
   "Next", **Then** a message indicates all comments have been reviewed.
3. **Given** a comment with existing replies in its thread, **When** the comment is
   displayed in one-by-one view, **Then** all thread replies are shown beneath the
   original comment in chronological order.

---

### User Story 3 - Reply to a Review Comment (Priority: P2)

From either the list view or the one-by-one view, the reviewer can compose and submit
a reply to any comment thread. The reply is posted to the pull request on GitHub and
appears immediately in the thread within the app.

**Why this priority**: Responding to reviewers is a core part of the review workflow;
without replies the tool is read-only.

**Independent Test**: Reply to one comment on a real or sandbox PR and verify the
reply appears in the GitHub PR and is reflected in the app thread.

**Acceptance Scenarios**:

1. **Given** a displayed comment, **When** the user types a reply and submits,
   **Then** the reply is posted to the GitHub PR thread and the thread in the app
   updates to show the new reply.
2. **Given** a reply compose area, **When** the user submits an empty reply,
   **Then** submission is blocked and a validation message is shown.
3. **Given** a failed submission (e.g., network error), **When** the post attempt
   fails, **Then** the composed text is preserved and an error message prompts the
   user to retry.

---

### User Story 4 - Resolve a Review Comment (Priority: P2)

The reviewer marks an individual comment thread as resolved. The resolved comment is
visually distinguished in the list view and is excluded from the one-by-one navigation
queue by default.

**Why this priority**: Tracking which comments have been addressed is essential for
knowing when a PR is ready for re-review.

**Independent Test**: Mark one comment as resolved and confirm it is visually
distinguished in the list and skipped in the one-by-one navigation queue.

**Acceptance Scenarios**:

1. **Given** an unresolved comment, **When** the user resolves it,
   **Then** the comment thread is marked resolved on GitHub and the app updates its
   visual state accordingly.
2. **Given** the summary list with resolved comments hidden by default, **When** the
   user toggles "Show resolved", **Then** resolved comments appear with a distinct
   resolved indicator.
3. **Given** one-by-one navigation, **When** a comment has been resolved,
   **Then** it is skipped in the navigation sequence unless the user opts to include
   resolved comments.

---

### User Story 5 - Commit a Suggested Change (Priority: P3)

When a reviewer has left a suggestion (a GitHub suggestion block), the user can accept
and commit that suggestion directly from the app without leaving to the GitHub web
interface. The committed change is applied to the PR branch.

**Why this priority**: Suggestion commits reduce round-trips to the browser; they are
valuable but the app is usable without this capability.

**Independent Test**: On a PR with a GitHub suggestion comment, accept the suggestion
from the app and verify the commit appears on the PR branch.

**Acceptance Scenarios**:

1. **Given** a comment containing a GitHub-format code suggestion, **When** the user
   chooses "Commit suggestion", **Then** the suggestion is committed to the PR branch
   and a confirmation message shows the commit SHA.
2. **Given** a comment without a suggestion block, **When** the comment is displayed,
   **Then** no "Commit suggestion" control is shown.
3. **Given** a suggestion commit that fails (e.g., merge conflict), **When** the error
   occurs, **Then** the app displays a clear error and the branch state is unchanged.

---

### User Story 6 - Manage Ignored Commenters (Priority: P3)

The user maintains a list of commenter usernames (typically CI bots) whose comments are
automatically hidden from the list view and one-by-one navigation. The user can add,
view, and remove entries from an app settings area.

**Why this priority**: CI noise is a common pain point but not a blocker for the core
review workflow.

**Independent Test**: Add a username to the ignored list, open a PR where that user has
comments, and verify those comments do not appear in any view.

**Acceptance Scenarios**:

1. **Given** the settings screen, **When** the user adds a GitHub username to the
   ignored list and returns to a PR, **Then** comments authored by that username are
   hidden from all comment views.
2. **Given** an ignored commenter in the list, **When** the user removes them,
   **Then** their comments reappear in subsequent PR views.
3. **Given** the ignored list, **When** the user opens settings, **Then** all currently
   ignored usernames are displayed and each can be individually removed.

---

### Edge Cases

- What happens when a PR has zero review comments? The app MUST display an empty-state
  message rather than a blank or broken UI.
- What happens when the user's GitHub token lacks permission to resolve a thread or post
  a reply? The app MUST show a clear permission-error message and not silently fail.
- What happens when a PR has hundreds of comments (e.g., 500+)? The list MUST remain
  responsive; comments MUST be paginated or virtualized.
- What happens when the GitHub API is unreachable? The app MUST display a
  connection-error state and allow the user to retry.
- What happens when a suggestion conflicts with recent commits? The app MUST surface the
  conflict error before any commit is attempted.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The app MUST display all review comments for a selected pull request in a
  summary list view showing author, file path, and comment excerpt.
- **FR-002**: The app MUST support one-by-one navigation through review comments,
  showing full comment text and surrounding diff context per comment.
- **FR-003**: Users MUST be able to reply to any comment thread from within the app;
  replies MUST be posted to GitHub and reflected immediately in the thread.
- **FR-004**: Users MUST be able to mark a comment thread as resolved; the resolution
  MUST be reflected on GitHub.
- **FR-005**: The app MUST support accepting and committing GitHub-format code
  suggestions directly to the PR branch.
- **FR-006**: Users MUST be able to configure a persistent list of ignored commenter
  usernames; comments from ignored users MUST be hidden across all views.
- **FR-007**: The app MUST clearly distinguish resolved comments from unresolved ones
  in the list view.
- **FR-008**: One-by-one navigation MUST skip resolved comments by default, with a
  user-accessible option to include them.
- **FR-009**: The app MUST display an empty-state message when a PR has no review
  comments.
- **FR-010**: The app MUST handle GitHub API errors gracefully with user-facing error
  messages and retry options.

### Key Entities

- **Pull Request**: A GitHub PR identified by owner, repo, and PR number; has a title,
  status, branch name, and associated comments.
- **Review Comment**: A comment on a specific file line or PR-level; has author, body,
  file path, line number, thread ID, and resolved status.
- **Comment Thread**: A collection of a root comment and its replies; can be resolved
  or unresolved as a unit.
- **Suggestion**: A special comment body block containing a proposed code change that
  can be committed directly to the PR branch.
- **Ignored Commenter**: A stored GitHub username whose comments are filtered from all
  views; persisted across app sessions.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A reviewer can open a PR with 50 comments and see the full summary list
  rendered in under 3 seconds.
- **SC-002**: Navigating between comments in one-by-one mode takes under 300
  milliseconds per transition.
- **SC-003**: 100% of submitted replies and resolutions are reflected on GitHub within
  the same session without requiring a manual app restart.
- **SC-004**: Committing a suggestion completes within 5 seconds from the moment the
  user confirms the action.
- **SC-005**: Adding or removing an ignored commenter takes effect on the current PR
  view without requiring an app restart.
- **SC-006**: The app remains responsive when displaying a PR with 200+ review
  comments, with no perceptible freeze during scrolling or navigation.

## Non-Functional Requirements

### Auth & Token Security

- **NFR-001**: The app MUST authenticate exclusively via GitHub OAuth 2.0 Device Flow.
  Personal Access Token (PAT) input is out of scope for v1.
- **NFR-002**: The GitHub OAuth scope used is `repo`. The narrower `public_repo` scope
  is insufficient because the GraphQL API mutations required to resolve/unresolve comment
  threads require write access to private repositories.
- **NFR-003**: The OAuth token MUST be stored in the OS native keychain only (via
  `go-keyring`). It MUST NOT be written to disk, logged, or transmitted to the frontend.
- **NFR-004**: If the OS keychain is unavailable (e.g., no `libsecret` daemon on Linux),
  the app MUST display a clear error message explaining the dependency and exit gracefully.
- **NFR-005**: When a stored token is revoked or has expired, the app MUST detect the
  `401 Unauthorized` response from the GitHub API, clear the stored token, and redirect
  the user to the auth screen with an explanatory message.

### Environment

- **NFR-006**: The app requires the environment variable `GITURA_GITHUB_CLIENT_ID` to be
  set to the GitHub OAuth App's client ID. If absent at startup, the app MUST log an
  error and fail to start with a descriptive message.
- **NFR-007**: On Linux, the app requires `libwebkit2gtk-4.0` and `libsecret-1` to be
  installed on the host system. These are documented prerequisites.

### API

- **NFR-008**: GitHub API pagination MUST be handled transparently. The page size for
  review comment fetches is 100 (GitHub maximum). All pages MUST be fetched before
  rendering.
- **NFR-009**: GitHub API rate limit errors (`403` with `X-RateLimit-Remaining: 0`) MUST
  be surfaced to the user with a distinct message that includes the reset time.
- **NFR-010**: GraphQL resolve/unresolve mutations require the comment thread's `node_id`
  (a `PullRequestReviewThread` global ID). The `node_id` MUST be fetched as part of the
  initial PR load flow and cached in-memory.

### UI State

- **NFR-011**: Resolve/unresolve actions MUST apply optimistic UI updates: the local
  state updates immediately, the GitHub API call is made asynchronously, and if the API
  call fails, the local state is rolled back with an error message.
- **NFR-012**: After adding or removing an ignored commenter, the current PR view MUST
  update reactively (no reload required) to show or hide the affected comments and
  recalculate counts.

### Performance

- **NFR-013**: The diff render target is ≤ 500ms for a 5,000-line diff file. This is
  a hard budget; virtualization or lazy rendering MUST be used if the target cannot
  be met with a full-render approach.
- **NFR-014**: Memory usage MUST remain below 150MB (peak, measured with the Go runtime
  profiler) for a single PR session with up to 200 comment threads.
- **NFR-015**: Comment list virtualization MUST be applied when the rendered thread count
  exceeds 50. Pagination is also acceptable if virtualization is not feasible in the
  chosen framework.

## Assumptions

- The app targets desktop platforms (macOS, Linux, Windows); mobile support is out of
  scope for v1.
- Only pull request review comments (line-level and file-level) are in scope; general
  PR-level conversation comments (the main PR conversation) are out of scope for v1.
  This is communicated to users via an informational note in the PR input UI.
- The ignored-commenter list is stored locally on the user's machine and is not synced
  across devices.
- The app requires an active internet connection; offline mode is out of scope for v1.
- The code diff context displayed alongside a comment is sourced from the `diff_hunk`
  field returned per comment by the GitHub REST API and does not require a local clone
  of the repository.
- A single PR is loaded and reviewed at a time; multi-PR batch workflows are out of
  scope for v1. The UI does not prevent loading a second PR — doing so replaces the
  current PR in memory after a confirmation prompt if changes are in-flight.

## Additional Requirements

### PR Input (CHK003, CHK004)

- **FR-011**: The PR input form MUST accept either a full GitHub PR URL
  (`https://github.com/{owner}/{repo}/pull/{number}`) or three separate fields
  (owner, repo, number). URL parsing MUST extract all three components. Invalid formats
  MUST display a validation error message.
- **FR-012**: After loading a PR, the app MUST display the PR state (`open`, `closed`,
  `merged`). For `closed` or `merged` PRs, comment actions (reply, resolve, commit
  suggestion) MUST still be available but the app MUST show a non-blocking warning
  banner indicating the PR is no longer open.

### Review Comment Scope (CHK002)

- **FR-013**: "Review comments" in this app refer exclusively to GitHub Pull Request
  Review Comment objects (`/repos/{owner}/{repo}/pulls/{pull_number}/comments` REST
  endpoint, `PullRequestReviewComment` GraphQL type). General PR-level issue comments
  (`/repos/{owner}/{repo}/issues/{issue_number}/comments`) are out of scope.

### Loading States (CHK009)

- **FR-014**: Every async operation MUST have a defined loading state:
  - PR load: progress bar driven by `pr:load-progress` event.
  - Thread list refresh: spinner on the list container.
  - Reply submit: button disabled + spinner; text preserved in compose area.
  - Resolve/unresolve: button disabled + spinner; optimistic update (NFR-011).
  - Suggestion commit: modal with progress indicator; button disabled.

### Error Categories (CHK019)

- **FR-015**: The frontend MUST parse error string prefixes to determine display
  category: `auth:` → auth error toast with re-login CTA; `github:` → API error toast
  with retry button; `validation:` → inline form error; `keyring:` → modal error with
  setup instructions; `notfound:` → empty state with descriptive message.

### Device Flow Expiry (CHK006)

- **FR-016**: When the `auth:device-flow-expired` Wails event is received, the frontend
  MUST stop polling, display a message that the device code has expired, and show a
  button to restart the device flow.

### Show-Resolved Toggle Persistence (CHK007)

- **FR-017**: The "Show resolved" toggle state MUST persist for the duration of the
  current PR session. It MUST reset to `false` (hidden) when a new PR is loaded.

### Settings Error Handling (CHK008)

- **FR-018**: If `GetIgnoredCommenters()` fails because the settings file is corrupt or
  unreadable, the app MUST display an error message and treat the list as empty (not
  crash). The user MUST be able to add commenters to a fresh list in this state.

### Suggestion Scope (CHK039, CHK042)

- **FR-019**: If the target file of a suggestion has been deleted or renamed since the
  suggestion was made, `CommitSuggestion` MUST return a `github:` prefixed error and the
  app MUST display the error without attempting the commit.
- **FR-020**: Multi-file suggestions are out of scope for v1. A suggestion that spans
  multiple files is displayed as a suggestion block but the "Commit suggestion" action is
  disabled with a tooltip explaining the limitation.

### Concurrent Conflict Scenarios (CHK034, CHK035, CHK036)

- **FR-021**: If resolving or replying to a thread returns a `404` or `422` from GitHub
  (indicating the thread may have been resolved or deleted by another user), the app
  MUST refresh the affected thread's state from the API and display a descriptive error.

### Duplicate Ignored Commenter (CHK043)

- **FR-022**: When `AddIgnoredCommenter` is called for a username already on the list,
  the operation silently succeeds (no error shown, no duplicate added). The UI MUST
  prevent the user from seeing duplicate entries by disabling or hiding the "Add" control
  for already-ignored usernames.

### Comment Display Clarity (CHK011, CHK012, CHK041)

- **FR-023**: Comment excerpts in the list view MUST be truncated at 200 characters with
  a trailing ellipsis. The full body is shown in the detail view.
- **FR-024**: Diff context in the detail view MUST show the full `diff_hunk` returned by
  the GitHub API (typically ±4 lines). No additional truncation is applied.
- **FR-025**: In the detail view, long comment bodies MUST be displayed in a scrollable
  container capped at 400px height.

### Resolved Visual Treatment (CHK014)

- **FR-026**: Resolved comment threads MUST be displayed with a muted opacity (≤ 50%)
  and a "Resolved" badge in the list view when shown via the "Show resolved" toggle.

### Optimistic Update Semantics (CHK013, CHK066)

- **FR-027**: Reply submission uses a server-confirmed update: the reply is appended to
  the thread in the UI only after the GitHub API returns a success response. The composed
  text is preserved during the in-flight period.
- **FR-028**: Resolve/unresolve uses an optimistic local update (NFR-011).

### Navigation Queue Semantics (CHK023)

- **FR-029**: The one-by-one navigation queue is ordered by file path then line number.
  Resolved threads are excluded by default. The "Show resolved" toggle in the list view
  also controls whether resolved threads appear in the navigation queue.

### Count Scope (CHK024)

- **FR-030**: `CommentCount` and `UnresolvedCount` on the PR summary reflect only review
  comments (FR-013) after the ignored-commenter filter is applied.

### Ignore Filter Consistency (CHK025)

- **FR-031**: The ignored-commenter filter is applied in the Go backend at query time for
  both `GetCommentThreads` and `LoadPullRequest` (for counts). It is NOT applied
  client-side, ensuring identical filter semantics across list view and navigation queue.
