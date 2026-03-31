# Research: PR Deep Review Workflow

**Feature**: 004-pr-review-workflow  
**Date**: 2026-03-31  
**Status**: Complete — extends spec 001 research; all new unknowns resolved  
**Basis**: All spec 001 decisions (UI framework, component library, Go↔Vue comms, auth,
go-github client, keyring, distribution) are **confirmed unchanged**. This document
records only decisions that are new or clarified for feature 004.

---

## Spec 001 Decisions — Confirmed

| Decision | Spec | Status |
|---|---|---|
| Wails v2 + Vue 3 + TypeScript frontend | 001 §1 | Confirmed |
| shadcn-vue + Tailwind CSS v4 | 001 §2 | Confirmed |
| Wails first-class bindings (App struct public methods) | 001 §3 | Confirmed |
| OAuth Device Flow, scope `repo` | 001 §4 | Confirmed |
| `go-github/v67` REST client | 001 §5 | Confirmed |
| `go-keyring` for token storage | 001 §6 | Confirmed |
| `wails build` per-platform, GitHub Actions matrix | 001 §7 | Confirmed |
| Suggestion commit via Git Contents API (PUT `/contents/{path}`) | 001 §8 | Confirmed |

---

## 1. Review Thread Loading — GitHub GraphQL API

**Decision**: Use GitHub GraphQL API to fetch review threads, resolved state, and thread
node IDs. Raw HTTP POST to `https://api.github.com/graphql` using the existing auth token
via `net/http` + `encoding/json`. No new Go module dependency.

**Rationale**: The GitHub REST API (`GET /repos/{owner}/{repo}/pulls/{number}/comments`)
returns individual comments but does NOT expose thread-level `resolved` state or the
`PullRequestReviewThread` node ID required for resolve/unresolve GraphQL mutations.
The REST PR Reviews endpoint (`GET .../reviews`) returns review summaries, not threaded
comment groups. Thread resolve state and thread node IDs are exclusively available via
the GraphQL API's `pullRequest.reviewThreads` query.

**GraphQL query shape**:
```graphql
query FetchReviewThreads($owner: String!, $repo: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 50, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          comments(first: 100) {
            nodes {
              databaseId
              body
              author { login avatarUrl }
              path
              line
              originalLine
              diffHunk
              createdAt
              url
              replyTo { databaseId }
            }
          }
        }
      }
    }
  }
}
```

**Pagination**: Paginate review threads using `pageInfo.hasNextPage` + `endCursor`.
Up to 50 threads per page (GraphQL default max). Comments within a thread: 100 per page
(adequate for any realistic thread length).

**Alternatives considered**:
- `shurcooL/githubv4` typed GraphQL client: clean but adds a new dependency; the query
  shape is stable enough for a hand-rolled struct.
- REST only: impossible — thread `resolved` state and thread node IDs are not in the
  REST API.

---

## 2. Resolve / Unresolve Thread — GraphQL Mutations

**Decision**: Use GraphQL mutations `resolveReviewThread` and `unresolveReviewThread` via
the same raw HTTP GraphQL transport established in §1.

**Mutation shapes**:
```graphql
mutation ResolveThread($threadId: ID!) {
  resolveReviewThread(input: { threadId: $threadId }) {
    thread { id isResolved }
  }
}

mutation UnresolveThread($threadId: ID!) {
  unresolveReviewThread(input: { threadId: $threadId }) {
    thread { id isResolved }
  }
}
```

**Thread ID source**: The `PullRequestReviewThread.id` field from the FetchReviewThreads
query (§1). Stored as `CommentThreadDTO.NodeID` in the backend cache.

**Optimistic UI strategy**: The frontend sets the thread's `resolved` state immediately on
user action (optimistic); the backend mutation fires asynchronously. On mutation error,
the backend returns an error and the frontend rolls back to the previous state.

**Alternatives considered**:
- REST endpoint for resolve: GitHub does not expose a REST endpoint for PR review thread
  resolution. The `PUT /repos/.../pulls/comments/{id}/resolve` documented in some
  third-party docs does not exist in the current GitHub API surface.

---

## 3. In-App Navigation — Review View Entry/Exit

**Decision**: Extend the existing `currentPage` ref in `App.vue` to include `'review'`.
Pass the selected `PRListItem` as a prop to `ReviewPage.vue`. Use Vue's `<KeepAlive>`
around `PRPage` to preserve its internal state (scroll position, filter state) while the
review view is active.

**Navigation flow**:
1. User clicks a PR row in `PRPage.vue` → emits `open-review` event with `PRListItem`
2. `App.vue` sets `selectedPRItem = item` and `currentPage = 'review'`
3. `ReviewPage.vue` receives `prItem` prop; calls `LoadPullRequest(owner, repo, number)`
4. User clicks back → emits `close-review` from `ReviewPage.vue`
5. `App.vue` sets `currentPage = 'pr'`; PRPage is already mounted (KeepAlive) with
   full state intact — no re-fetch, scroll position preserved

**Scroll position**: `<KeepAlive>` preserves Vue component instance including all refs
and reactive state. The DOM scroll position of a scrollable container inside a kept-alive
component IS preserved because the DOM node is not destroyed. No manual save/restore needed
provided the scrollable element is inside the component's template (not `body`).

**Filter state**: `usePRFilters` composable state is already scoped to `PRPage.vue`; with
`<KeepAlive>` the component instance is preserved, so filter state requires no extra work.

**App.vue template pattern**:
```html
<KeepAlive>
  <PRPage v-if="currentPage === 'pr'" @open-review="handleOpenReview" />
</KeepAlive>
<ReviewPage
  v-if="currentPage === 'review'"
  :pr-item="selectedPRItem"
  @close-review="handleCloseReview"
/>
```

**Alternatives considered**:
- Vue Router with `<RouterView>` and scroll behavior: adds router dependency; existing
  pattern is in-app state switching, no router.
- Manual scroll save/restore in `onActivated` / `onDeactivated` hooks: works but
  unnecessary given `<KeepAlive>` DOM preservation for within-component scrollables.

---

## 4. Hybrid Data Strategy — PR Metadata vs. Comment Threads

**Decision**: When navigating to the review view:
1. **Immediate display** — `PRListItem` fields (title, owner, repo, number, html_url,
   is_draft) are passed as props and rendered without waiting for an API call.
2. **Fresh fetch** — `LoadPullRequest(owner, repo, number)` is called on mount to get
   the full `PullRequestSummary` (including current `State` for the status banner) and
   to populate the in-memory comment thread cache via `FetchReviewThreads`.

The "hybrid" benefit: title, branch, and number are visible instantly; the loading state
only affects the comment list and status banner.

**Note**: `PRListItem` does not carry PR `State` (open/closed/merged) as `ListOpenPRs`
only returns open PRs. `LoadPullRequest` fetches the fresh PR state for the status banner.
In practice, a PR navigated to from the list will almost always be open, but the banner
correctly handles edge cases (merged/closed race between list fetch and review load).

---

## 5. Reply to Comment — REST API

**Decision**: Post replies via the GitHub REST API using go-github:
`POST /repos/{owner}/{repo}/pulls/{number}/comments` with `in_reply_to` set to the
root comment ID. This is a typed go-github method:
`client.PullRequests.CreateComment(ctx, owner, repo, number, &github.PullRequestComment{...})`

**Reply draft preservation** (FR-017): The reply textarea content is stored in a `ref`
in `ReplyComposer.vue`. On API error, the component is not unmounted — the draft
string remains in the input. No special persistence needed.

---

## 6. Suggestion Commit Detail — Confirmed

**Decision**: Confirmed from spec 001 §8 — use the Git Contents API:
1. `GET /repos/{owner}/{repo}/contents/{path}?ref={head_branch}` — get file content + blob SHA
2. Apply the suggestion diff patch to the file content (string replacement of hunk lines)
3. `PUT /repos/{owner}/{repo}/contents/{path}` — commit the replacement

**SHA conflict detection**: The blob SHA fetched in step 1 must match the blob SHA
computed from the in-memory file content. If the file has been modified since the review
loaded, abort with `github:conflict` error.

**Suggestion extraction**: Parse comment body for ` ```suggestion` fenced block. Lines in
the diff hunk above the suggestion block are the "original" lines; suggestion block content
is the "replacement".

---

## 7. Ignored Commenters — Local JSON Persistence

**Decision**: Confirmed from spec 001 data-model. Storage path:
`os.UserConfigDir()/gitura/ignored_commenters.json`. Simple JSON array of
`IgnoredCommenterDTO`. Create directory if absent. Atomic write via temp file + rename.

**Filter application**: Server-side in Go. When `GetCommentThreads` is called,
`FetchReviewThreads` results are filtered: any thread where ALL comments are by ignored
authors is excluded entirely; individual replies by ignored authors within a mixed thread
are excluded from the `Comments` slice. If only the root comment is by an ignored author
but replies are not, the thread is collapsed to show only the non-ignored replies.

> **Clarification**: For simplicity in v1, entire threads are excluded when the root
> comment author is ignored. This aligns with FR-003 intent (hide noise) and avoids
> complex partial-thread display logic.

---

## Constitution Check — Post-Design

| Principle | Status | Notes |
|---|---|---|
| I. Code Quality (gofmt, golint, cyclomatic ≤ 10) | PASS | GraphQL helpers split into focused functions; suggestion commit split into fetch/patch/push steps |
| II. Testing (80%/90% coverage, fixtures, TDD) | PASS | GraphQL responses fixture-recorded via `httptest`; suggestion commit has SHA-conflict test case |
| III. UX Consistency (single design language, ARIA) | PASS | All new components use shadcn-vue primitives; diff view keyboard nav + ARIA roles |
| IV. Performance (load ≤ 2 s, diff ≤ 500 ms, mem < 150 MB) | PASS | GraphQL single-query thread load; diff hunk is plain text rendered in `<pre>`; no virtualization needed at 200 threads |
| Technology Stack | PASS | No new Go module deps; `net/http` + `encoding/json` already in stdlib |
| Development Workflow | PASS | Screenshots required for all new Vue components |
