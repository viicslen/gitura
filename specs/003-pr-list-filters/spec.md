# Feature Specification: Open PR List with Filters

**Feature Branch**: `003-pr-list-filters`  
**Created**: 2026-03-31  
**Status**: Draft  
**Input**: Replace the placeholder pr list page with an actual implementation which list currently open prs for the user, allowing filters for repo, org, date, author, etc

## Clarifications

### Session 2026-03-31

- Q: When a filter changes, does the list update from cached data (client-side) or via a new API query (server-side)? → A: Each filter change triggers a new GitHub API search query.
- Q: Should author/assignee/reviewer involvement types always be combined, or can the user choose which to show? → A: Default to all combined; user can toggle involvement types individually.
- Q: When the GitHub API rate limit is hit, what should happen? → A: Show an error message with the rate limit reset time.
- Q: When a user clicks a PR row, what should happen? → A: Navigate to the in-app PR review/detail view.
- Q: Should draft PRs be included in the list by default? → A: Exclude drafts by default; user can enable an "include drafts" toggle.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View All Open PRs (Priority: P1)

An authenticated user opens the PR page and immediately sees a list of all open pull requests involving them — across all repositories — so they can quickly assess what needs attention.

**Why this priority**: This is the core value of the feature. Without a working list, nothing else is useful. It replaces the current placeholder and delivers immediate visibility into open work.

**Independent Test**: Can be fully tested by navigating to the PR page while authenticated and verifying that open PRs are shown with correct metadata (title, repo, author, age), delivering actionable awareness of open work.

**Acceptance Scenarios**:

1. **Given** a user is authenticated, **When** they navigate to the PR page, **Then** a list of currently open pull requests appears showing at minimum: PR title, repository name, author, and time since opened.
2. **Given** the user has no open PRs involving them, **When** they navigate to the PR page, **Then** an appropriate empty state message is displayed.
3. **Given** the user has many open PRs, **When** the list loads, **Then** results are shown in a default order of most recently updated first and the user can scroll through all of them.

---

### User Story 2 - Filter by Repository (Priority: P2)

A user who works across many repositories wants to narrow the PR list to a specific repository so they can focus on one codebase at a time.

**Why this priority**: Without scoping, high-volume users are overwhelmed. Repository filtering is the single most commonly needed scope reduction.

**Independent Test**: Can be tested independently by selecting a repository from a filter control and verifying that only PRs from that repository appear in the list.

**Acceptance Scenarios**:

1. **Given** the PR list is loaded, **When** the user selects a specific repository from the repository filter, **Then** only PRs belonging to that repository are shown.
2. **Given** a repository filter is active, **When** the user clears the filter, **Then** PRs from all repositories are shown again.

---

### User Story 3 - Filter by Organization (Priority: P2)

A user who belongs to multiple GitHub organizations wants to scope the PR list to a single organization to reduce noise from unrelated work.

**Why this priority**: Organizations provide a higher-level scope that is especially valuable for contractors or members of many orgs — conceptually parallel to repository filtering in value.

**Independent Test**: Can be tested independently by selecting an organization filter and verifying only PRs from repositories within that organization appear.

**Acceptance Scenarios**:

1. **Given** the PR list is loaded, **When** the user selects an organization from the org filter, **Then** only PRs from repositories owned by that organization are shown.
2. **Given** an org filter and a repository filter are both active, **When** the list renders, **Then** only PRs matching both constraints are shown (filters use AND logic).

---

### User Story 4 - Filter by Author (Priority: P3)

A team lead or reviewer wants to see open PRs by a specific author — for example, to review work from a particular teammate or to check their own submissions.

**Why this priority**: Useful for review-focused workflows, but secondary to repo/org scoping. Most users will rely on default scoping (their own involvement) without needing author filtering.

**Independent Test**: Can be tested by entering a GitHub username in the author filter and verifying that only PRs authored by that user appear in the list.

**Acceptance Scenarios**:

1. **Given** the PR list is loaded, **When** the user enters or selects a GitHub username in the author filter, **Then** only PRs authored by that user are shown.
2. **Given** an author filter is active, **When** the user clears it, **Then** PRs from all authors are shown again.

---

### User Story 5 - Filter by Date Range (Priority: P3)

A user wants to find PRs opened or updated within a specific time window — for example, PRs from the last 7 days — to focus on recent or overdue work.

**Why this priority**: Date filtering is a secondary refinement. Most users get sufficient value from repo/org filtering; date ranges are useful for audit or sprint-scoping workflows.

**Independent Test**: Can be tested by selecting a preset date range and verifying that only PRs opened or updated within that range appear in the list.

**Acceptance Scenarios**:

1. **Given** the PR list is loaded, **When** the user selects a date range filter (e.g., "last 7 days"), **Then** only PRs opened or updated within that period are shown.
2. **Given** no PRs fall within a selected date range, **When** the filter is applied, **Then** an empty state message is displayed indicating no results match the filter.

---

### Edge Cases

- When the GitHub API rate limit is reached, the system MUST display an error message stating that the limit has been reached and showing the time at which the limit resets; the user must manually retry after that time. Manual retry is initiated by clicking the existing refresh button — no additional retry control is required.
- How does the system handle a user with membership in dozens of organizations and hundreds of open PRs?
- What is displayed if the user's authentication token expires mid-session while the list is displayed?
- What if a filter combination yields zero results — is it clear to the user that filters (not an empty account) are the cause?
- **Resolved**: When the user applies a filter while the initial PR list is loading, the in-flight initial request is superseded — the new filtered query replaces it, and the loading state continues without interruption. Stale responses from the superseded request are discarded; only the most recent query's result is displayed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The PR page MUST display a list of currently open pull requests relevant to the authenticated user. By default, all three involvement types are shown together: PRs authored by the user, PRs where the user is an assignee, and PRs where the user is a requested reviewer.
- **FR-001a**: The user MUST be able to toggle each involvement type (author / assignee / reviewer) on or off independently; deselecting a type removes those PRs from the active query. At least one involvement type must remain active at all times.
- **FR-002**: Each PR entry in the list MUST show at minimum: PR title, repository name (owner/repo), author login, and time elapsed since the PR was opened. Each entry MUST be clickable and navigate the user to the in-app PR review/detail view for that pull request.
- **FR-003**: The PR list MUST be filterable by repository. Repository options are populated by querying GitHub for all repos in which the user has open PRs matching the current involvement filters, presented as a searchable dropdown. Typing a partial name narrows the options in the dropdown.
- **FR-004**: The PR list MUST be filterable by organization, showing only PRs from repositories owned by a user-selected organization.
- **FR-005**: The PR list MUST be filterable by author, showing only PRs authored by a user-specified GitHub login.
- **FR-006**: The PR list MUST be filterable by date, allowing users to restrict results to PRs opened or updated within a chosen time window (e.g., last 7 days, last 30 days, or a custom range).
- **FR-007**: Multiple filters MUST be composable — applying two or more filters simultaneously narrows results using AND logic (all active criteria must match). Involvement types (author / assignee / reviewer) are combined with OR logic among themselves before the AND is applied with other filters. When the selected repository is not owned by the selected organization, the result set is empty; no error is shown, and the empty-state message indicates that active filters produced no results.
- **FR-008**: Each active filter MUST be individually clearable without affecting other active filters.
- **FR-013**: When any filter value changes, the system MUST issue a new GitHub API search query incorporating all currently active filters; results displayed always reflect the server response to the most recent query. Any in-flight request from a prior filter state MUST be superseded — stale responses are silently discarded.
- **FR-013a**: Rapid filter changes (e.g. typing in a text field) MUST be debounced (300 ms) to avoid issuing a query on every keystroke. Toggle changes (involvement types, include-drafts) fire immediately without debounce.
- **FR-009**: The system MUST display a meaningful empty state when no PRs match the current filter combination, distinguishing between "no PRs exist for this account" and "no PRs match the active filters."
- **FR-010**: The PR list MUST display in a default order of most recently updated first.
- **FR-011**: The system MUST handle and surface errors gracefully when the PR list cannot be loaded (e.g., network failure, expired token), with a user-actionable message.
- **FR-014**: When the GitHub API rate limit is exhausted, the system MUST display an error message that includes the time at which the rate limit resets; no automatic retry is performed — the user must manually trigger a refresh after the reset time.
- **FR-015**: Draft pull requests MUST be excluded from the list by default. The user MUST be able to enable an "include drafts" toggle to include them; toggling it triggers a new API query.
- **FR-012**: Filter state MUST persist within the user's session — navigating away from and back to the PR page retains the last-used filter combination.

### Key Entities

- **Pull Request**: An open PR with title, number, repository, author, creation date, last-updated date, draft status, and review/assignment status relative to the current user.
- **Repository**: A GitHub repository identified by owner and name; serves as a filterable dimension.
- **Organization**: A GitHub organization owning one or more repositories; serves as a higher-level filterable scope.
- **Author**: A GitHub user identified by login who created a pull request.
- **Filter State**: The current combination of active filters (repo, org, author, date range) applied to the PR list view.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can see their open PRs within 3 seconds of navigating to the PR page on a standard broadband connection.
- **SC-002**: Applying or changing a filter issues a new API query and displays updated results within 3 seconds on a standard broadband connection.
- **SC-003**: Users can reduce a list of 50+ open PRs to a targeted subset using one or more filters in under 30 seconds.
- **SC-004**: The PR page shows 100% of PR-related fields (title, author, repo, date) populated from live data — no placeholder content remains.
- **SC-005**: Filter combinations produce correct results with 0% false positives — no PR appears in the list that does not match all active filter criteria.

## Assumptions

- The user is authenticated via the existing GitHub OAuth device flow before accessing the PR list.
- "Open PRs relevant to the user" defaults to all three involvement types combined: authored by the user, assigned to the user, and review-requested from the user. The user can toggle any involvement type off, but cannot deselect all three simultaneously.
- The initial PR list is fetched from GitHub's API; pagination is handled automatically to return complete results within the session.
- Organization membership is derived from the authenticated user's GitHub account — no additional org configuration is required in the app.
- Repository filter options are populated from the repos that appear in the fetched PR list (not all repos the user has access to), keeping filter choices contextually relevant.
- Filter state is maintained in-memory for the session duration; it is not persisted across application restarts.
- Mobile/responsive layout is out of scope; the feature targets the desktop app form factor only.
- Archived or disabled repositories are excluded from filter options.
