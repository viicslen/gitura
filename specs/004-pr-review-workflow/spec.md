# Feature Specification: PR Deep Review Workflow

**Feature Branch**: `004-pr-review-workflow`  
**Created**: 2026-03-31  
**Status**: Draft  
**Input**: User description: "PR review experience — when a user clicks a PR from the list, they are taken to a full review view for that pull request. The view shows all review comments in a summary list (author, file, excerpt). From there the user can navigate comments one-by-one, seeing the full comment body and surrounding diff context. The user can reply to any comment thread, mark threads as resolved or unresolved, and accept and commit GitHub suggestion blocks directly to the PR branch. Draft PRs and closed/merged PRs can still be loaded with a status banner. Users can manage a persistent ignored-commenters list (e.g. CI bots) so those comments are hidden across all views. This is the implementation of the deep review workflow that was designed in spec 001 but never built."

## Clarifications

### Session 2026-03-31

- Q: When the user selects a PR from the PR list, how is PR data passed to the review view? → A: Hybrid — PR metadata (title, status, number, branch) is passed from the list cache; review comments and thread node_ids are always fetched fresh when the review view loads.
- Q: Should a "Show resolved" toggle be included per spec 001 semantics? → A: Yes — resolved threads are hidden from the summary list and navigation queue by default; a toggle in the list view reveals them; the toggle resets to hidden when a new PR is loaded.
- Q: What is the end-of-navigation-queue behavior when the user presses Next on the last comment? → A: Show an "All comments reviewed" end-state message and disable forward navigation at that point.
- Q: When the user navigates back from the review view to the PR list, what state is restored? → A: Full restore — scroll position and all active filter state are preserved with no re-fetch of list data.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Review Comment Summary (Priority: P1)

A developer clicks a pull request from the PR list and is taken to a dedicated review view. The view displays all review comments in a scrollable summary list, where each entry shows the comment author, the file it targets, and a short excerpt of the comment body. The developer can see at a glance how many comments exist and who wrote them, giving them immediate orientation before diving in.

**Why this priority**: This is the entry point for the entire review workflow. Without it, no other review interactions are possible. It delivers standalone value as a read-only review summary.

**Independent Test**: Can be fully tested by clicking any open PR and verifying the review view loads with a comment list showing author, file, and excerpt for each review comment.

**Acceptance Scenarios**:

1. **Given** the user is on the PR list, **When** they click a PR, **Then** the app navigates to a review view showing a summary list of all review comments for that PR.
2. **Given** the review view is loaded, **When** the PR has review comments, **Then** each entry in the list displays the comment author's name, the file path the comment targets, and a short excerpt (first 200 characters, truncated with ellipsis) of the comment body.
3. **Given** the review view is loaded, **When** the PR has no review comments, **Then** the view shows an empty state message indicating there are no review comments.
4. **Given** the review view is loaded, **When** a comment belongs to the ignored-commenters list, **Then** that comment is excluded from the summary list.
5. **Given** the review view is loaded with resolved threads present, **When** the "Show resolved" toggle is off (default), **Then** resolved threads are hidden from the summary list.
6. **Given** the "Show resolved" toggle is turned on, **When** resolved threads exist, **Then** they appear in the list with a distinct visual treatment (muted styling and a "Resolved" badge).
7. **Given** a new PR is loaded, **When** the review view initializes, **Then** the "Show resolved" toggle resets to its default off state regardless of its prior value.
8. **Given** the user is in the review view, **When** they navigate back to the PR list, **Then** the list is displayed with its previous scroll position and active filter state intact and no new data fetch is triggered.

---

### User Story 2 - Comment-by-Comment Navigation with Diff Context (Priority: P2)

From the comment summary list, the developer selects a comment to inspect it in detail. The view transitions to a focused comment panel showing the full comment body and the surrounding diff hunk for the file and line the comment was left on. The developer can step forward and backward through all comments without returning to the summary list.

**Why this priority**: This is the core review interaction. Reading comments alongside their diff context is the primary reason a developer opens a PR for review. It is independently valuable even without reply or resolve actions.

**Independent Test**: Can be fully tested by clicking a comment in the summary list and verifying the full comment body and diff hunk are shown, then using next/previous navigation to step through all comments.

**Acceptance Scenarios**:

1. **Given** the comment summary list is shown, **When** the user selects a comment, **Then** the full comment body is displayed alongside the diff hunk for the relevant file and line range.
2. **Given** the detail panel is open, **When** the user navigates to the next comment, **Then** the panel updates to show the next comment's body and its associated diff context.
3. **Given** the detail panel is open on the last comment in the queue, **When** the user presses Next, **Then** an "All comments reviewed" message is shown and forward navigation is disabled.
4. **Given** the detail panel is open, **When** the user navigates to the previous comment, **Then** the panel updates to show the previous comment in the list.
5. **Given** the diff context is rendered, **When** the comment targets a specific line, **Then** that line is visually highlighted within the diff hunk.
6. **Given** the "Show resolved" toggle is off, **When** the user navigates through comments, **Then** resolved threads are skipped in the navigation sequence.
7. **Given** the "Show resolved" toggle is on, **When** the user navigates through comments, **Then** resolved threads are included in the navigation sequence.

---

### User Story 3 - Reply to and Resolve Comment Threads (Priority: P3)

While reviewing a comment, the developer can type a reply directly in the detail panel and submit it to the PR thread. They can also mark any comment thread as resolved or unresolved, which updates the thread state on the remote service and is reflected immediately in the summary list.

**Why this priority**: Responding and resolving are the core review collaboration actions. Without them the tool is read-only. They are bundled at P3 because full diff-context navigation (P2) is a prerequisite for effective replies.

**Independent Test**: Can be fully tested by submitting a reply to an existing PR comment thread and verifying the reply appears remotely, then marking the thread resolved and verifying the summary list updates.

**Acceptance Scenarios**:

1. **Given** a comment detail panel is open, **When** the user types a reply and submits it, **Then** the reply is posted to the PR thread and the thread is updated in the view.
2. **Given** a comment detail panel is open, **When** the user clicks "Resolve", **Then** the thread is marked resolved remotely and the summary list entry is updated to reflect the resolved state.
3. **Given** a resolved comment thread, **When** the user clicks "Unresolve", **Then** the thread is marked unresolved remotely and the summary list updates accordingly.
4. **Given** the user submits a reply, **When** the submission fails (e.g., network error), **Then** an error message is shown and the draft reply is preserved so it can be retried.
5. **Given** resolved threads exist and "Show resolved" is off, **When** the summary list is displayed, **Then** resolved threads are hidden; when "Show resolved" is on, resolved threads appear with muted styling and a "Resolved" badge.

---

### User Story 4 - Accept and Commit GitHub Suggestion Blocks (Priority: P4)

When a review comment contains a GitHub suggestion block (a proposed code change), the developer can accept the suggestion directly from the detail panel. The suggestion is committed to the PR branch without leaving the app.

**Why this priority**: Suggestion acceptance is a high-value productivity feature that eliminates the round-trip to the GitHub web UI. It is ranked P4 because it requires the comment detail panel (P2) as a prerequisite and applies only to a subset of comments.

**Independent Test**: Can be fully tested by opening a PR that has at least one suggestion comment, accepting the suggestion from the detail panel, and verifying a new commit appears on the PR branch with the suggested change applied.

**Acceptance Scenarios**:

1. **Given** a comment detail panel is open and the comment contains a suggestion block, **When** the view renders the comment, **Then** an "Accept Suggestion" action is visible alongside the comment body.
2. **Given** the "Accept Suggestion" action is clicked, **When** the commit is created, **Then** a commit is pushed to the PR branch applying the suggested change, and the comment's suggestion block is marked as applied.
3. **Given** a comment has no suggestion block, **When** the detail panel renders the comment, **Then** no "Accept Suggestion" action is shown.
4. **Given** the suggestion commit fails (e.g., merge conflict), **When** the error occurs, **Then** a descriptive error message is shown and no partial commit is pushed.

---

### User Story 5 - Status Banner for Non-Open PRs (Priority: P5)

When the developer opens a draft, closed, or merged PR from the list, the review view loads normally but displays a prominent status banner at the top indicating the PR's current state. All review comments remain visible and interactive, but a clear visual indicator distinguishes these PRs from active open ones.

**Why this priority**: Allowing review of non-open PRs prevents accidental confusion about PR state without blocking access to historical review content. It is P5 because it is a safety and clarity concern, not a core workflow blocker.

**Independent Test**: Can be fully tested by opening a draft PR and a merged PR from the list and verifying a banner is displayed for each that accurately reflects its state, while comments remain visible.

**Acceptance Scenarios**:

1. **Given** the user opens a draft PR, **When** the review view loads, **Then** a status banner reading "Draft" (or equivalent) is shown at the top of the view.
2. **Given** the user opens a merged PR, **When** the review view loads, **Then** a status banner reading "Merged" is shown.
3. **Given** the user opens a closed (not merged) PR, **When** the review view loads, **Then** a status banner reading "Closed" is shown.
4. **Given** a non-open PR is loaded, **When** the user interacts with the view, **Then** all comments, navigation, and reply actions remain accessible (no interactions are disabled solely because of PR state).

---

### User Story 6 - Ignored-Commenters Management (Priority: P6)

The developer can maintain a persistent list of commenter usernames (e.g., CI bots, automated reviewers) whose comments are hidden across all PR review views. They can add usernames to the list, see the current list, and remove entries. The ignored state is stored locally and persists across app restarts.

**Why this priority**: Ignored-commenters is a quality-of-life feature that reduces noise in active codebases with many bot reviewers. It does not block any core review workflow and is therefore the lowest priority in this feature.

**Independent Test**: Can be fully tested by adding a bot username to the ignored list, then opening a PR where that bot has comments, and verifying those comments do not appear in the summary list. Restarting the app and verifying the list persists confirms storage.

**Acceptance Scenarios**:

1. **Given** the user opens the ignored-commenters settings, **When** they enter a GitHub username and save it, **Then** the username is added to the ignored list and persisted locally.
2. **Given** a username is on the ignored list, **When** any PR review view loads, **Then** all comments authored by that username are excluded from the summary list and detail navigation.
3. **Given** the user has entries on the ignored list, **When** they view the list, **Then** all current ignored usernames are displayed with an option to remove each.
4. **Given** the user removes a username from the ignored list, **When** they navigate to a PR review view, **Then** comments from that username are visible again.
5. **Given** the app is restarted, **When** the user opens any PR review view, **Then** the previously configured ignored-commenters list is still applied.

---

### Edge Cases

- What happens when a PR has hundreds of review comments — does the summary list remain performant and scrollable?
- What happens when the diff context for a comment cannot be retrieved (e.g., the file was deleted or the base commit is unavailable)?
- What happens when a suggestion block conflicts with other accepted suggestions on the same file?
- How does the view behave when the user loses network connectivity mid-review (e.g., after loading comments but before submitting a reply)?
- What happens when a comment thread is resolved by another reviewer while the current user has it open?
- What happens when the ignored list contains a username that matches the authenticated user's own username?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST display a dedicated PR review view when the user selects a PR from the PR list.
- **FR-001a**: On navigation to the review view, the system MUST use the PR metadata (title, status, number, branch) already available from the PR list; it MUST fetch review comments and thread node_ids fresh from the remote service before rendering the comment summary list.
- **FR-001b**: When the user navigates back from the review view to the PR list, the list MUST restore its previous scroll position and all active filter state without re-fetching list data.
- **FR-002**: System MUST present all review comments for the selected PR in a summary list showing author name, target file path, and a short excerpt of the comment body.
- **FR-003**: System MUST exclude comments authored by usernames on the ignored-commenters list from all review views.
- **FR-004**: System MUST allow the user to select any comment from the summary list to view its full body and the surrounding diff hunk in a detail panel.
- **FR-005**: System MUST provide forward and backward navigation controls to step through all non-ignored comments from within the detail panel; resolved threads MUST be excluded from the navigation sequence when the "Show resolved" toggle is off; reaching the last comment in the queue and pressing Next MUST display an "All comments reviewed" end-state message with forward navigation disabled.
- **FR-005a**: System MUST include a "Show resolved" toggle in the summary list view; the toggle MUST default to off (hidden) on each new PR load and reactively update the list and navigation queue without requiring a reload.
- **FR-006**: System MUST highlight the specific line(s) targeted by the comment within the displayed diff hunk.
- **FR-007**: System MUST allow the user to submit a text reply to any comment thread from the detail panel.
- **FR-008**: System MUST allow the user to mark any open comment thread as resolved, updating the state on the remote PR.
- **FR-009**: System MUST allow the user to mark any resolved comment thread as unresolved, updating the state on the remote PR.
- **FR-010**: System MUST visually distinguish resolved threads from unresolved threads when "Show resolved" is on (muted opacity and "Resolved" badge); resolved threads MUST be hidden from the list when the toggle is off.
- **FR-011**: System MUST detect suggestion blocks within comment bodies and display an "Accept Suggestion" action for those comments.
- **FR-012**: System MUST commit an accepted suggestion to the PR branch and mark the suggestion as applied.
- **FR-013**: System MUST show a clear status banner at the top of the review view when the loaded PR is in draft, closed, or merged state.
- **FR-014**: System MUST allow the user to add GitHub usernames to a persistent ignored-commenters list via an accessible settings entry point.
- **FR-015**: System MUST allow the user to view and remove entries from the ignored-commenters list.
- **FR-016**: System MUST persist the ignored-commenters list across app sessions (survives restarts).
- **FR-017**: System MUST preserve unsent reply drafts and display an error message when a reply submission fails.
- **FR-018**: System MUST display a descriptive error message when a suggestion commit cannot be applied (e.g., conflict or network failure).

### Key Entities

- **PR Review View**: The full-screen view representing one pull request's review session; contains the comment summary list and detail panel.
- **Review Comment**: A single inline or file-level comment on the PR, with attributes: author, target file, target line range, body text, thread state (resolved/unresolved), and optional suggestion block.
- **Comment Thread**: A logical grouping of a root review comment and its replies; carries a resolved/unresolved state.
- **Suggestion Block**: A structured code proposal embedded in a review comment; can be accepted and committed to the PR branch.
- **Diff Hunk**: The excerpt of the file diff surrounding the line(s) a review comment targets; provides context for reading the comment.
- **Ignored-Commenters List**: A user-owned, locally persisted list of GitHub usernames whose comments are filtered from all review views.
- **PR Status**: The current lifecycle state of a pull request — open, draft, closed, or merged — displayed as a banner for non-open PRs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can open a PR review view and see all non-ignored comments within 3 seconds of selecting a PR.
- **SC-002**: Users can navigate from one comment to the next (with full body and diff context loaded) in under 1 second per transition.
- **SC-003**: Users can submit a reply to a comment thread, and the reply is confirmed as posted within 5 seconds under normal network conditions.
- **SC-004**: Users can accept a suggestion and receive confirmation that the commit was pushed within 10 seconds under normal network conditions.
- **SC-005**: 100% of comments authored by ignored usernames are absent from all review views without requiring manual filtering per-PR.
- **SC-006**: The ignored-commenters list survives an app restart with no data loss.
- **SC-007**: Draft, closed, and merged PRs display the correct status banner on every load with no false positives on open PRs.
- **SC-008**: Review views remain functional and responsive with up to 200 review comments on a single PR.

## Assumptions

- Users are already authenticated; no new authentication flow is required for this feature.
- The PR list (built in spec 003) is the navigation entry point; this feature adds the review view as the destination when a PR is clicked.
- When entering the review view, PR metadata (title, status, number, branch) is taken from the list's in-memory cache; review comments and thread node_ids are fetched fresh from the remote service on each view load.
- Suggestion acceptance creates a single commit per suggestion; batch-accepting multiple suggestions in one commit is out of scope for this iteration.
- The ignored-commenters list is stored locally on the user's machine; it is not synced remotely or shared across devices.
- All PR states (draft, closed, merged, open) are determined from the remote service response and require no additional polling beyond the initial PR load.
- Mobile and tablet support is out of scope; this is a desktop application.
- The diff hunk displayed is the hunk already associated with the review comment by the remote service; the app does not independently parse raw diffs.
- Reply submission covers replying to existing comment threads only; submitting a new top-level review or review summary is out of scope.
- Resolved/unresolved state management reflects and writes remote state; no local overrides are maintained.
