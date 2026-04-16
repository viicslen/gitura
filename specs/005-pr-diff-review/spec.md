# Feature Specification: PR Diff Review

**Feature Branch**: `005-pr-diff-review`  
**Created**: 2026-04-01  
**Status**: Draft  
**Input**: User description: "PR diff review — from a PR detail view, the user can review the actual code changes file by file. Changed files are shown in a sidebar list and can also be stepped through sequentially with next/previous navigation. The diff is displayed in a split/side-by-side format with unchanged sections collapsed (GitHub-style, expandable). The user can leave inline comments on a single line or a range of multiple lines. Comments can be posted immediately as standalone comments, or added to a pending review. When in review mode, pending comments are posted to GitHub immediately as draft comments and are batched until the user submits the review with a verdict: Approve, Request Changes, or Comment. Existing review comment threads from other reviewers are hidden by default but the user can toggle them on to see them anchored inline on their diff lines. The view must be performant — files are loaded on demand (not all at once), and large files collapse unchanged hunks to avoid rendering thousands of lines unnecessarily."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Switch Between PR Views (Priority: P1)

When viewing a pull request, the reviewer needs a way to move between the conversation/comments view and the diff review view. A multi-value toggle (e.g., a segmented control or tab bar) is present at the top of the PR, showing at minimum "Conversation" and "Files changed" as options. The control is designed to accommodate additional views in the future without restructuring. The reviewer's current position within each view is preserved when they switch away and return.

**Why this priority**: The toggle is the entry point into the diff review view — it gates access to everything else in this feature. It also preserves the usability of the existing conversation view by making the two views co-equal rather than replacing one with the other.

**Independent Test**: Can be fully tested by verifying the toggle control appears on a PR detail page with both view options, switching between them, and confirming each view renders its correct content with state preserved.

**Acceptance Scenarios**:

1. **Given** a PR detail page is open, **When** the reviewer views the page, **Then** a multi-value view toggle is visible showing at least "Conversation" and "Files changed" options.
2. **Given** the toggle is visible, **When** the reviewer selects "Files changed", **Then** the diff review view is displayed.
3. **Given** the reviewer is in the diff review view, **When** they select "Conversation", **Then** the conversation/comments view is displayed.
4. **Given** the reviewer has scrolled partway through a file diff, **When** they switch to "Conversation" and back to "Files changed", **Then** the diff view restores to where they left off.
5. **Given** the toggle currently shows "Conversation" as active, **When** it renders, **Then** the active option is visually distinguished from inactive options.
6. **Given** a third view is added in a future iteration, **When** it is configured, **Then** the toggle accommodates it as a new option without requiring a different navigation component.

---

### User Story 2 - Browse and Navigate Changed Files (Priority: P2)

A reviewer opens a pull request and switches to the "Files changed" view. They see a sidebar listing all changed files (with file path, change type indicator, and line count). They can click any file to jump directly to its diff, or step through files in order using previous/next navigation buttons. Unchanged sections of each file are collapsed by default so only meaningful changes are visible; the reviewer can expand any collapsed hunk to see surrounding context.

**Why this priority**: This is the foundational capability of the diff review view — without file navigation and a readable diff, no other review actions are possible. It delivers standalone value as a read-only code review tool.

**Independent Test**: Can be fully tested by opening a PR with multiple changed files, verifying the sidebar lists all files, clicking through them, and confirming the split diff renders with collapsed unchanged hunks.

**Acceptance Scenarios**:

1. **Given** a PR with 10 changed files is open, **When** the reviewer enters the diff review view, **Then** a sidebar lists all 10 files with their paths and change status (added/modified/deleted).
2. **Given** the sidebar is visible, **When** the reviewer clicks a file name, **Then** the diff for that file is displayed immediately without loading other files.
3. **Given** a file diff is displayed, **When** the reviewer views it, **Then** unchanged code sections are collapsed to a single expandable bar showing the number of hidden lines.
4. **Given** a collapsed hunk is visible, **When** the reviewer clicks the expand control, **Then** the hidden lines are revealed inline.
5. **Given** the reviewer is on a file diff, **When** they click "Next file", **Then** the next file in the sidebar list is loaded and displayed; "Previous file" navigates back.
6. **Given** the reviewer is on the last file, **When** they click "Next file", **Then** the button is disabled or shows an end-of-review indicator.

---

### User Story 3 - Leave Inline Comments (Priority: P3)

A reviewer spots an issue on a specific line and wants to leave a comment. They can click on a single diff line to open a comment form, or click and drag across multiple lines to select a range. They then type their comment and choose to either post it immediately as a standalone comment (visible on GitHub right away) or add it to a pending review batch.

**Why this priority**: Inline commenting is the primary review action — reading diffs without the ability to comment is incomplete for a review tool, but it can be built and verified independently of the formal review submission flow.

**Independent Test**: Can be fully tested by selecting a line on any diff, typing a comment, and posting it immediately; then verifying it appears on the corresponding GitHub PR.

**Acceptance Scenarios**:

1. **Given** a diff line is visible, **When** the reviewer hovers or clicks the line, **Then** a comment affordance (e.g., a "+" icon) becomes visible on that line.
2. **Given** the comment affordance is clicked, **When** the comment form opens, **Then** the form is anchored to that specific line in the diff.
3. **Given** the reviewer clicks and drags across multiple consecutive diff lines, **When** they release, **Then** the comment form opens referencing the entire selected line range.
4. **Given** the comment form is open with text entered, **When** the reviewer chooses "Add single comment", **Then** the comment is posted immediately to GitHub and appears anchored to that line in the diff.
5. **Given** the comment form is open with text entered, **When** the reviewer chooses "Start a review" or "Add review comment", **Then** the comment is submitted to GitHub as a draft review comment and review mode is activated.
6. **Given** review mode is active, **When** the reviewer adds another inline comment, **Then** it is also added as a draft comment to the same pending review.

---

### User Story 4 - Submit a Formal Review with Verdict (Priority: P4)

Once the reviewer has added all their draft comments, they submit the review by choosing a verdict: Approve, Request Changes, or Comment. An optional top-level review body can accompany the verdict. On submission, all pending draft comments are published and GitHub records the review event.

**Why this priority**: Formal review submission is the culmination of the review workflow. It depends on P2 (inline comments) but is independently testable once pending comments are in place.

**Independent Test**: Can be tested by entering review mode, adding at least one draft comment, then submitting with each of the three verdict options and verifying the GitHub PR reflects the correct review state.

**Acceptance Scenarios**:

1. **Given** review mode is active with at least one pending comment, **When** the reviewer opens the "Submit review" panel, **Then** they see three verdict options: Approve, Request Changes, Comment.
2. **Given** the reviewer selects "Approve" and clicks Submit, **When** the action completes, **Then** GitHub records an approved review, all pending comments are published, and the UI confirms success.
3. **Given** the reviewer selects "Request Changes" with a body comment, **When** submitted, **Then** GitHub records a changes-requested review with the body attached.
4. **Given** review mode is active, **When** the reviewer discards the review, **Then** the reviewer is warned before discarding; existing draft comments on GitHub remain as drafts unless explicitly deleted.
5. **Given** no pending comments exist, **When** the reviewer submits with "Approve" or "Request Changes", **Then** submission proceeds with just the verdict and optional body.

---

### User Story 5 - View Other Reviewers' Comment Threads (Priority: P5)

Existing review comment threads left by other reviewers are hidden by default to keep the reviewer's focus on the code. The reviewer can toggle a control to reveal all existing threads, which appear anchored inline at their respective diff lines. The reviewer can read them for context.

**Why this priority**: This is a contextual enhancement. The review workflow is complete without it, but seeing prior feedback prevents duplicate comments and helps reviewers understand prior discussions.

**Independent Test**: Can be tested by opening a PR that already has comments from other reviewers, toggling the "Show reviewer comments" control, and verifying threads appear at the correct diff lines.

**Acceptance Scenarios**:

1. **Given** a PR has existing review comment threads from other users, **When** the reviewer opens the diff view, **Then** those threads are hidden by default.
2. **Given** existing threads are hidden, **When** the reviewer toggles "Show reviewer comments", **Then** all existing threads become visible, each anchored to its diff line.
3. **Given** threads are visible and the reviewer navigates to a file, **When** the diff renders, **Then** existing threads for that file appear inline at their correct line positions.
4. **Given** threads are visible, **When** the reviewer toggles the control off, **Then** all threads are hidden without affecting the diff view.

---

### Edge Cases

- What happens when a file is too large to render all lines — only changed hunks and a fixed context window are shown; the remainder stays collapsed.
- What happens when a file is binary — a placeholder is shown indicating the file cannot be diffed; size change is shown if available.
- What happens when a file is renamed with no content changes — the sidebar shows the rename and the diff view displays old/new paths with no line-level changes.
- What happens when the reviewer loses connectivity mid-review — pending local state is preserved; the reviewer is informed and can retry submission.
- What happens when the PR is merged or closed while the reviewer has it open — a notice is shown; review submission remains possible if GitHub permits it.
- What happens when a reviewer's thread references a line that no longer exists (stale/outdated thread) — the thread is shown with an "outdated" visual indicator positioned at its best available context position.
- What happens when the reviewer tries to comment on a line hidden inside a collapsed hunk — the hunk must first be expanded; the collapsed bar does not support direct commenting.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The PR detail page MUST display a multi-value view toggle (e.g., segmented control or tab bar) that allows switching between available PR views.
- **FR-002**: The view toggle MUST show at minimum two options: the conversation/comments view and the "Files changed" diff review view.
- **FR-003**: The view toggle MUST be extensible — new view options can be added in future iterations without replacing or restructuring the toggle component.
- **FR-004**: The active view MUST be visually indicated in the toggle so the reviewer always knows which view they are in.
- **FR-005**: Switching views MUST preserve the reviewer's position and state in each view; returning to a view restores it to where they left off.
- **FR-006**: The system MUST display a sidebar listing all changed files in the PR, each showing the file path, change type (added/modified/deleted/renamed), and lines changed count.
- **FR-007**: The system MUST allow the reviewer to navigate between files sequentially using previous/next controls that step through the sidebar file list in order.
- **FR-008**: The system MUST display diffs in a side-by-side (split) format with the previous version on the left and the new version on the right.
- **FR-009**: The system MUST collapse unchanged code sections within a file diff by default, replacing them with a single expandable bar that indicates the number of hidden lines.
- **FR-010**: The reviewer MUST be able to expand any collapsed hunk to reveal the hidden lines in-place within the diff.
- **FR-011**: Files MUST be loaded on demand — only the currently viewed file's diff is fetched and rendered; other files are not pre-loaded.
- **FR-012**: The reviewer MUST be able to select a single diff line to initiate an inline comment on that line.
- **FR-013**: The reviewer MUST be able to select a contiguous range of diff lines to attach a single inline comment to the entire span.
- **FR-014**: The reviewer MUST be able to post an inline comment immediately as a standalone comment, making it visible on GitHub at once.
- **FR-015**: The reviewer MUST be able to add an inline comment to a pending review batch; doing so activates review mode.
- **FR-016**: When review mode is active, each added comment MUST be submitted to GitHub immediately as a draft review comment.
- **FR-017**: The reviewer MUST be able to submit the pending review with one of three verdicts: Approve, Request Changes, or Comment.
- **FR-018**: The review submission panel MUST accept an optional top-level review body message.
- **FR-019**: On review submission, all pending draft comments MUST be published and the GitHub review event MUST be recorded.
- **FR-020**: Existing review comment threads from other reviewers MUST be hidden by default when the diff view is opened.
- **FR-021**: The reviewer MUST be able to toggle visibility of all other reviewers' comment threads via a single control.
- **FR-022**: When revealed, existing comment threads MUST be anchored inline at their corresponding diff lines.
- **FR-023**: Stale (outdated) comment threads MUST be visually distinguished from current threads.
- **FR-024**: Binary files and files that cannot be diffed MUST show a placeholder with available metadata rather than failing silently or showing an error.

### Key Entities

- **Pull Request**: The parent entity identified by owner, repo, and PR number; contains the list of changed files and existing reviews.
- **Changed File**: A file within the PR diff; has a path, change type, patch content (hunks), and associated review threads.
- **Diff Hunk**: A contiguous block of changed lines within a file; includes surrounding context lines. Unchanged sections between hunks are collapsed by default.
- **Diff Line**: A single line in a hunk; has a side (left/right), line number, and content. Serves as the anchor point for inline comments.
- **Inline Comment**: A comment attached to a specific diff line or line range within a file; can be a standalone comment or a draft review comment.
- **Pending Review**: A batched set of draft comments accumulated before a verdict is submitted. Associated with the current reviewer and PR.
- **Review**: A submitted review event on GitHub with a verdict (approve/request-changes/comment), an optional body, and zero or more inline comments.
- **Review Thread**: A review comment thread anchored to a specific line/range in a file; may be from the current reviewer or from others; can be current or outdated.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Reviewers can open any changed file and see its diff within 2 seconds of selecting it, regardless of the total number of files in the PR.
- **SC-002**: Inline comment creation — from clicking a line to having a ready-to-type comment form — takes under 1 second.
- **SC-003**: A complete review cycle (navigate files, add comments, submit verdict) can be accomplished in under 5 minutes for a PR with up to 10 files and 200 changed lines.
- **SC-004**: The diff view renders the first file within 1 second of entering the review view, without blocking on loading all other files.
- **SC-005**: Unchanged code is collapsed by default so that a file with 500+ unchanged lines does not require the reviewer to scroll past them; only changed hunks and a limited context window are visible initially.
- **SC-006**: Toggling other reviewers' comment threads on or off takes effect instantly (under 300 ms) with no visible loading delay after the review view has been opened.
- **SC-007**: No draft review comments are silently lost; all pending comments survive navigating between files within the review session.

## Assumptions

- The reviewer is authenticated and has read access to the repository; no new authentication mechanism is required.
- This feature extends the existing PR detail view; the diff review view is accessed via a view toggle on the PR detail page, alongside the existing conversation view.
- Only the current reviewer's own inline comments can be added in this view; replying to others' threads is out of scope.
- Real-time collaborative review (multiple reviewers editing simultaneously in the same session) is out of scope.
- Desktop viewport is the primary target; responsive/mobile layout is out of scope for this iteration.
- Syntax highlighting within diffs is a progressive enhancement; the feature must function correctly without it.
- Comment editing and deletion from within this view are out of scope; those actions remain on GitHub's own interface.
- The number of files in a PR is expected to be in the typical range (up to ~100 files); extremely large PRs with 500+ files are not a primary optimization target for this iteration.
- The GitHub API provides sufficient diff data to reconstruct side-by-side view and anchor comment threads to line positions.
