# Implementation Quality Checklist: PR Review UI

**Purpose**: Validate that requirements are complete, clear, consistent, and measurable before committing implementation — author self-review pass covering all 6 user stories.
**Created**: 2026-03-30
**Feature**: [spec.md](../spec.md) · [plan.md](../plan.md) · [contracts/wails-bindings.md](../contracts/wails-bindings.md) · [data-model.md](../data-model.md)
**Scope**: US0 (Auth) · US1 (List) · US2 (Navigate) · US3 (Reply) · US4 (Resolve) · US5 (Suggestion) · US6 (Settings)
**Audience**: Author self-review before committing / PR creation

---

## Requirement Completeness

- [x] CHK001 - Are PR-level conversation comments explicitly excluded from scope, and is this communicated to users in the UI requirements? **Resolution**: Added FR-013 to spec.md explicitly scoping review comments to the GitHub PR Review Comment REST type; FR-011 notes an informational UI message will explain the exclusion. [Completeness, Spec §Assumptions]
- [x] CHK002 - Are requirements defined for what constitutes a "review comment" vs. a PR-level comment — specifically which GitHub API object type is in scope? **Resolution**: FR-013 defines review comments as `PullRequestReviewComment` objects from the `/repos/{owner}/{repo}/pulls/{pull_number}/comments` endpoint; general issue comments are explicitly out of scope. [Completeness, Gap]
- [x] CHK003 - Is the PR input mechanism (owner/repo/number form vs. URL parse) fully specified, including format validation rules? **Resolution**: FR-011 added to spec.md: accepts full GitHub PR URL or three separate fields; URL parsing extracts all three components; invalid formats trigger a validation error. [Completeness, Gap]
- [x] CHK004 - Are requirements specified for the PR state display (open/closed/merged) and whether the app should warn or block actions on closed/merged PRs? **Resolution**: FR-012 added: PR state displayed after load; non-blocking warning banner shown for closed/merged PRs; actions remain available. [Completeness, Gap]
- [x] CHK005 - Are all three Wails events (`auth:device-flow-complete`, `auth:device-flow-expired`, `pr:load-progress`) fully specified with payload types and all consumer behaviors? **Resolution**: All three events documented in contracts/wails-bindings.md §Events with payload types. FR-016 covers `auth:device-flow-expired` UI behavior; FR-014 covers `pr:load-progress` progress bar. [Completeness, Contracts §Events]
- [x] CHK006 - Is `auth:device-flow-expired` event handling specified in the UI requirements? It is listed in the contracts but absent from spec user stories. **Resolution**: FR-016 added to spec.md: on receiving the event, stop polling, show expired message, display restart button. Go `PollDeviceFlow` now emits the event on expired status. `useAuth.ts` now handles the event. [Completeness, Gap, Contracts §Events]
- [x] CHK007 - Are requirements defined for the "Show resolved" toggle's persistence — should it survive page navigation or reset each session? **Resolution**: FR-017 added: toggle persists for current PR session, resets to `false` when a new PR is loaded. [Completeness, Gap, Spec §FR-008]
- [x] CHK008 - Are requirements specified for what happens when `GetIgnoredCommenters()` is called and the settings file is corrupt or unreadable? **Resolution**: FR-018 added: display error, treat list as empty, allow adding to a fresh list. [Completeness, Gap]
- [x] CHK009 - Are loading state requirements defined for all async operations (PR load, thread list refresh, reply submit, resolve/unresolve, suggestion commit)? **Resolution**: FR-014 added with explicit loading state for each async operation. [Completeness, Gap]
- [x] CHK010 - Is the `pr:load-progress` event payload (`{ loaded: int, total: int }`) and its frontend display behavior (progress bar, text, etc.) fully specified? **Resolution**: Event payload documented in contracts/wails-bindings.md §Events; FR-014 specifies a progress bar driven by this event. [Completeness, Contracts §Events]

---

## Requirement Clarity

- [x] CHK011 - Is "comment excerpt" in FR-001 quantified with a character limit or line count? "Excerpt" is ambiguous without a defined truncation length. **Resolution**: FR-023 added: excerpts truncated at 200 characters with trailing ellipsis in list view; full body shown in detail view. [Clarity, Spec §FR-001]
- [x] CHK012 - Is "surrounding diff context" in FR-002 / US2 defined — specifically, how many lines above/below the comment line should be shown? **Resolution**: FR-024 added: full `diff_hunk` from GitHub API shown (typically ±4 lines); no additional truncation. [Clarity, Spec §FR-002, US2]
- [x] CHK013 - Is "reflected immediately" in FR-003 and FR-004 quantified? Does it mean optimistic UI update, or confirmed server round-trip before display update? **Resolution**: FR-027 (replies = server-confirmed) and FR-028 (resolve = optimistic) added to spec.md with explicit semantics. [Clarity, Spec §FR-003, FR-004]
- [x] CHK014 - Is "visually distinguished" in FR-007 and US4 defined with specific visual treatment (color, badge, opacity, strikethrough)? **Resolution**: FR-026 added: resolved threads shown at ≤ 50% opacity with a "Resolved" badge in list view. [Clarity, Spec §FR-007]
- [x] CHK015 - Is "remains responsive" in the 200+ comments edge case quantified with measurable criteria (e.g., scroll FPS, input latency)? **Resolution**: NFR-014 (< 150MB memory) and NFR-015 (virtualization at > 50 threads) added to spec.md; SC-006 retained as the latency criterion. [Clarity, Spec §Edge Cases, SC-006]
- [x] CHK016 - Is "clear error" in the suggestion-conflict edge case defined with a specific message format or error category prefix? **Resolution**: data-model.md §Suggestion specifies `github:conflict` error prefix for SHA conflicts; FR-015 maps `github:` prefix to an API error toast with retry. [Clarity, Spec §Edge Cases, Contracts §Error Codes]
- [x] CHK017 - Is the definition of "a GitHub suggestion block" (the ` ```suggestion ` fenced block detection rule) cross-referenced in the spec or only in data-model.md? **Resolution**: Spec §Additional Requirements §Suggestion Scope references the extraction rule in data-model.md; FR-019/FR-020 handle edge cases. [Clarity, Data-Model §Suggestion]
- [x] CHK018 - Is "takes effect on the current PR view without requiring an app restart" in SC-005 specified as a live reactive update (no reload) or a triggered refresh (user-initiated)? **Resolution**: NFR-012 added: live reactive update — no reload required; Go filter applied at query time. [Clarity, Spec §SC-005]
- [x] CHK019 - Is the error prefix convention (`auth:`, `github:`, `validation:`, `keyring:`, `notfound:`) referenced in spec/plan or only in the contracts? Frontend error handling requirements should cite it. **Resolution**: FR-015 added to spec.md with explicit prefix-to-display-behavior mapping for all five error categories. [Clarity, Contracts §Error Codes, Gap]

---

## Requirement Consistency

- [x] CHK020 - Does the `AuthState` DTO in the contracts include `Login` and `AvatarURL` as flat fields, while the model defines `User` as a nested struct? Is this discrepancy intentional and resolved? **Resolution**: Contracts win. `model.AuthState` updated to flat `Login + AvatarURL` string fields; nested `*User` removed. `app.go` `GetAuthState` updated. `models.ts` updated. `data-model.md` updated. [Consistency, Contracts §GetAuthState, Data-Model §User]
- [x] CHK021 - Does `CommentDTO` in the contracts (`AuthorLogin`, `AuthorAvatar` as strings) differ from `model.CommentDTO` which uses a nested `User` struct? Is the Go↔JS serialization contract consistent? **Resolution**: Contracts win. `model.CommentDTO` updated to flat `AuthorLogin + AuthorAvatar` string fields; nested `User` removed. `models.ts` updated. [Consistency, Contracts §GetCommentThreads, Data-Model §ReviewComment]
- [x] CHK022 - Is the `Resolved` field on `CommentDTO` in the contracts consistent with the data model, which defines resolved state at the `CommentThread` level (not per-comment)? **Resolution**: `Resolved bool` removed from `CommentDTO` in both `model.go` and `contracts/wails-bindings.md`. Resolved is thread-level only (`CommentThreadDTO.Resolved`). [Consistency, Contracts §GetCommentThreads, Data-Model §CommentThread]
- [x] CHK023 - Are navigation queue requirements in US2 and US4 consistent — US2 defines "skip resolved by default" and US4 extends it with a toggle; are these two specs aligned without contradiction? **Resolution**: FR-029 added: navigation queue uses same "Show resolved" toggle as list view; ordered by file path then line number. [Consistency, Spec §US2, US4, FR-008]
- [x] CHK024 - Does the spec's assumption "only review comments are in scope" align with the `LoadPullRequest` binding which returns `CommentCount` and `UnresolvedCount` — are these counts scoped to review comments only? **Resolution**: FR-030 added: both counts reflect only review comments (FR-013) after ignored-commenter filter is applied. [Consistency, Spec §Assumptions, Contracts §LoadPullRequest]
- [x] CHK025 - Are the ignored-commenter filter requirements consistent between US1 (list view) and US2 (navigation queue) — is the filter applied at the same data layer for both views? **Resolution**: FR-031 added: filter applied in Go backend at query time for all views; not applied client-side. [Consistency, Spec §US1 §AS2, US2, FR-006]

---

## Acceptance Criteria Quality

- [x] CHK026 - Can SC-001 ("50 comments rendered in under 3 seconds") be objectively measured, and is the measurement point defined (API response received vs. UI fully rendered)? **Resolution**: SC-001 measurement point is "UI fully rendered" (list visible and scrollable). No change to text needed; the spec's intent is sufficient for a manual benchmark test. [Measurability, Spec §SC-001]
- [x] CHK027 - Can SC-002 ("comment navigation transition ≤ 300ms") be measured — is the start event (button click) and end event (new comment fully rendered) specified? **Resolution**: Start = button click; end = new comment body and diff hunk fully rendered. Measurable via browser performance tools. [Measurability, Spec §SC-002]
- [x] CHK028 - Is SC-003 ("100% of replies/resolutions reflected on GitHub") measurable given optimistic UI updates — does "reflected" mean the GitHub API call succeeded or the UI updated? **Resolution**: SC-003 means the GitHub API call succeeded (not just UI updated). Optimistic updates are implementation detail; SC-003 covers the server confirmation. [Measurability, Spec §SC-003]
- [x] CHK029 - Is the US1 acceptance scenario "all 10 comments with author, file path, and excerpt" testable with a fixed PR, or dependent on live GitHub API state? **Resolution**: Test fixture-based — fixture PRs in `tests/fixtures/` provide stable API responses for all acceptance tests. No live API dependency in CI. [Measurability, Spec §US1 §AS1]
- [x] CHK030 - Does the acceptance scenario for US6 ("comments reappear in subsequent PR views") specify whether it means after a reload action or immediately reactively? **Resolution**: NFR-012 and FR-012 clarify: live reactive update. US6 AS2 "subsequent PR views" means the same session without reload (reactive). [Measurability, Spec §US6 §AS2]

---

## Scenario Coverage

- [x] CHK031 - Are requirements defined for the alternate flow where the user cancels the device flow mid-way (closes the auth dialog or clicks Cancel before authorizing)? **Resolution**: `stopPolling()` in `useAuth.ts` handles cancel; `logout()` clears state. Spec §FR-016 covers expiry; cancel is a subset (user stops polling explicitly). No additional spec change needed. [Coverage, Gap, Spec §US0]
- [x] CHK032 - Are requirements specified for token expiry after a successful login — what happens when a stored token becomes invalid mid-session? **Resolution**: NFR-005 added to spec.md: `401` from any GitHub API call triggers token deletion and redirect to auth screen with explanatory message. [Coverage, Gap, Spec §Assumptions]
- [x] CHK033 - Are requirements defined for navigating between PRs within a session — can the user load a second PR, and if so, is session state cleared? **Resolution**: Spec §Assumptions updated: loading a second PR replaces current PR in memory after a confirmation prompt if actions are in-flight. [Coverage, Gap, Spec §Assumptions]
- [x] CHK034 - Are requirements specified for the scenario where a thread is resolved by another user while the current user has it open in the detail view? **Resolution**: FR-021 added: 404/422 from resolve/reply triggers thread state refresh and descriptive error. [Coverage, Gap, Spec §US4]
- [x] CHK035 - Are requirements defined for concurrent suggestion commits — what if two users commit conflicting suggestions on the same file simultaneously? **Resolution**: SHA conflict detection documented in data-model.md §Suggestion; FR-019 handles deleted/renamed files. SHA pre-fetch before commit prevents stale-SHA commits. [Coverage, Gap, Spec §US5 §Edge Cases]
- [x] CHK036 - Are requirements specified for replying to a thread that has been resolved by another user between the user opening the thread and submitting the reply? **Resolution**: FR-021 covers this: 422 from GitHub on reply to a resolved thread triggers thread state refresh and descriptive error. [Coverage, Gap, Spec §US3]

---

## Edge Case Coverage

- [x] CHK037 - Is the zero-comments empty state specified for both the list view and the one-by-one navigation entry point separately? **Resolution**: Spec §Edge Cases already specifies empty-state message (FR-009). The spec applies to both views by FR-009's wording ("app MUST display an empty-state message"). [Edge Case, Spec §Edge Cases, FR-009]
- [x] CHK038 - Are requirements defined for a PR with comments where all threads are resolved — does the list show resolved threads, and what does one-by-one navigation display? **Resolution**: FR-008 already specifies one-by-one skips resolved by default with opt-in toggle. FR-017 and FR-029 make toggle behavior consistent. If all resolved and toggle is off, one-by-one shows the empty-reviewed state (US2 AS2). [Edge Case, Spec §US1, US2, FR-008]
- [x] CHK039 - Are requirements specified for a suggestion comment where the target file has been deleted or renamed since the suggestion was made? **Resolution**: FR-019 added: `CommitSuggestion` returns `github:` error; app displays error without attempting commit. [Edge Case, Gap, Spec §US5]
- [x] CHK040 - Are requirements defined for the ignored-commenter list when a username is empty or contains invalid GitHub username characters? **Resolution**: `AddIgnoredCommenter` contract already specifies "Returns error if `login` is empty." GitHub username validation (alphanumeric + hyphens, no leading hyphens) is enforced with a `validation:` error. [Edge Case, Data-Model §IgnoredCommenter, Contracts §AddIgnoredCommenter]
- [x] CHK041 - Are requirements specified for extremely long comment bodies — is there a display truncation limit in the list view and a scroll mechanism in the detail view? **Resolution**: FR-023 (200-char excerpt in list) and FR-025 (400px scrollable container in detail) added. [Edge Case, Gap, Spec §FR-001, FR-002]
- [x] CHK042 - Are requirements defined for a PR with a diff hunk that contains a suggestion spanning multiple files? (GitHub allows multi-file suggestions in some scenarios.) **Resolution**: FR-020 added: multi-file suggestions out of scope for v1; "Commit suggestion" disabled with explanatory tooltip. [Edge Case, Gap, Spec §US5]
- [x] CHK043 - Is the behavior specified when `AddIgnoredCommenter` is called for a username already on the ignored list — is the silent no-op behavior surfaced to the user or silently swallowed? **Resolution**: FR-022 added: silent success; UI prevents duplicate entry by disabling/hiding "Add" control for already-ignored usernames. [Edge Case, Contracts §AddIgnoredCommenter]

---

## Non-Functional Requirements

### Auth & Token Security

- [x] CHK044 - Is the OAuth scope (`repo`) justified in requirements, given that it grants full repository write access — is a narrower scope (e.g., `public_repo`) considered or explicitly rejected? **Resolution**: NFR-002 added to spec.md: `repo` scope required for GraphQL resolve/unresolve on private repos; `public_repo` explicitly rejected with justification. Also documented in plan.md. [Security, Gap, Spec §Assumptions]
- [x] CHK045 - Are requirements defined for token rotation or re-authentication prompts when the stored token is revoked by the user on GitHub? **Resolution**: NFR-005 added: `401` from any GitHub API call triggers token deletion and redirect to auth screen. [Security, Gap]
- [x] CHK046 - Is the security requirement for token storage (OS keychain only, never written to disk or logs) explicitly stated in the spec or plan? **Resolution**: NFR-003 added to spec.md; also documented in plan.md §Token Storage. [Security, Gap, Plan §Storage]
- [x] CHK047 - Are requirements specified for what happens if the OS keychain is unavailable (e.g., no unlock on Linux without `libsecret`)? **Resolution**: NFR-004 added to spec.md; also documented in plan.md §Keychain Unavailable. [Security, Edge Case, Plan §Constraints]

### GitHub API Contract Clarity

- [x] CHK048 - Are pagination requirements for `GetCommentThreads` specified — is there a defined page size, and are requirements for handling GitHub's 100-item `PerPage` maximum documented? **Resolution**: NFR-008 added: page size = 100 (GitHub maximum); all pages fetched before rendering. [API, Plan §Technical Context]
- [x] CHK049 - Are rate limit handling requirements defined — does the app need to surface rate limit errors distinctly from other GitHub API errors? **Resolution**: NFR-009 added: rate limit errors surfaced with distinct message including reset time. [API, Gap, Spec §FR-010]
- [x] CHK050 - Are requirements for the GraphQL mutations (resolve/unresolve) specified regarding which OAuth scope they require and what error to surface when the scope is missing? **Resolution**: NFR-002 (scope justification) and NFR-010 (node_id requirement) added. Scope errors surface as `auth:` prefix errors per FR-015. [API, Contracts §ResolveThread, Plan]
- [x] CHK051 - Is the requirement for `node_id` (needed for GraphQL resolve mutations) documented as a mandatory field in the PR load flow? **Resolution**: NFR-010 added to spec.md; `NodeID` field added to `CommentThreadDTO` in both `model.go` and `contracts/wails-bindings.md`; documented in `data-model.md §CommentThread`. [API, Contracts §ResolveThread, Data-Model §PullRequest]
- [x] CHK052 - Are requirements defined for SHA conflict detection in suggestion commits — specifically, should the app fetch the latest file SHA immediately before committing or rely on the cached PR head SHA? **Resolution**: data-model.md §Suggestion SHA conflict detection section added: live file SHA fetched immediately before commit; cached HeadSHA used only as reference. [API, Data-Model §Suggestion, Spec §US5 §Edge Cases]

### UI State Consistency

- [x] CHK053 - Are optimistic update requirements defined for resolve/unresolve — what is the rollback behavior if the GitHub API call fails after the UI has already updated? **Resolution**: NFR-011 added: optimistic update + rollback on failure with error message. [UI State, Spec §US4, Data-Model §CommentThread]
- [x] CHK054 - Are requirements specified for cache coherence between `prSummary.comment_count` and the actual rendered thread list after ignored-commenter changes? **Resolution**: NFR-012 added: reactive update after ignored-commenter changes recalculates counts. FR-031 specifies filter applied in Go at query time. [UI State, Contracts §AddIgnoredCommenter, Spec §FR-006]
- [x] CHK055 - Are transition animation or state requirements defined for switching between list view and detail view to prevent content flicker or layout shift? **Resolution**: FR-014 specifies spinner on list container during refresh; no explicit animation spec (implementation choice). Wails WebView handles layout consistency. [UI State, Gap, Spec §US1 §AS3]
- [x] CHK056 - Are keyboard navigation requirements specified for all interactive UI elements (list rows, next/prev buttons, reply form, resolve button, settings list)? **Resolution**: plan.md Constitution Check §III already calls out keyboard navigation as required (shadcn-vue provides accessible primitives). No additional spec item needed; shadcn-vue components are ARIA-compliant by default. [Accessibility, Plan §Constitution, FR-002]

### Performance

- [x] CHK057 - Is the diff render performance target (≤ 500ms for a 5,000-line file) sourced in the spec — it appears in plan.md but not in spec.md success criteria. Is this an implicit requirement? **Resolution**: NFR-013 added to spec.md: ≤ 500ms diff render target is a hard budget; virtualization/lazy rendering required if exceeded. [Performance, Plan §Performance Goals, Spec §Success Criteria, Gap]
- [x] CHK058 - Are memory requirements (< 150MB) defined as a hard limit or a guideline — and is the measurement method (peak, steady state, per PR) specified? **Resolution**: NFR-014 added: < 150MB peak measured with Go runtime profiler, for single PR session with up to 200 comment threads. [Performance, Plan §Performance Goals, Gap]
- [x] CHK059 - Are virtualization/pagination requirements for 200+ comment lists specified with a concrete threshold (e.g., "paginate when thread count > N")? **Resolution**: NFR-015 added: virtualization required when rendered thread count > 50; pagination is acceptable alternative. [Performance, Spec §Edge Cases, SC-006]

---

## Dependencies & Assumptions

- [x] CHK060 - Is the assumption that "code diff context is sourced from the GitHub API" validated — the GitHub REST API returns `diff_hunk` per comment, but is this sufficient for the full diff view specified in US2? **Resolution**: Spec §Assumptions updated: diff context is `diff_hunk` from GitHub REST API (typically ±4 lines); FR-024 specifies full hunk displayed; no local clone required. [Assumption, Spec §Assumptions, Data-Model §ReviewComment]
- [x] CHK061 - Is the Linux runtime dependency (`libwebkit2gtk` + `libsecret`) documented as a user-facing prerequisite in the quickstart or README? **Resolution**: NFR-007 added to spec.md; plan.md §Constraints already documents this. A quickstart.md will document it as a user-facing prerequisite when that phase is implemented. [Dependency, Plan §Constraints]
- [x] CHK062 - Is the `GITURA_GITHUB_CLIENT_ID` environment variable requirement documented in spec, plan, or quickstart — it is required at runtime but appears only in implementation? **Resolution**: NFR-006 added to spec.md; plan.md §Environment Variables section added. [Dependency, Gap]
- [x] CHK063 - Is the assumption that "a single PR is reviewed at a time" reflected in the UI requirements — is there a clear error or block if the user attempts to load a second PR without unloading the first? **Resolution**: Spec §Assumptions updated: loading a second PR replaces current PR after confirmation prompt if in-flight actions exist. [Assumption, Spec §Assumptions, Gap]

---

## Ambiguities & Conflicts

- [x] CHK064 - The spec states auth uses "personal access token or OAuth app" but the implementation uses Device Flow (OAuth app only). Is the PAT option intentionally dropped or still planned? **Resolution**: PAT option dropped for v1. Spec §Non-Functional Requirements NFR-001 updated: Device Flow only. Old Assumptions wording removed. plan.md §Auth Strategy documents this decision. [Ambiguity, Spec §Assumptions, Plan §Summary]
- [x] CHK065 - The contracts define `GetAuthState()` returning `Login` and `AvatarURL` as flat `AuthState` fields, but the model defines `AuthState.User` as a nested `*User` struct. This discrepancy needs resolution before frontend type bindings stabilize. **Resolution**: Contracts win. `model.AuthState` flattened to `Login + AvatarURL` string fields. `app.go`, `model.go`, `models.ts`, `data-model.md` all updated. [Conflict, Contracts §GetAuthState, Data-Model]
- [x] CHK066 - US3 requires replies be "reflected immediately in the thread within the app" but does not specify whether this is an optimistic local append or a re-fetch from GitHub. This ambiguity affects cache management strategy. **Resolution**: FR-027 added: replies use server-confirmed update (no optimistic append); text preserved during in-flight; thread updated after API success. [Ambiguity, Spec §US3 §AS1, FR-003]
- [x] CHK067 - The `auth:device-flow-expired` event is listed in contracts but has no corresponding spec user story or frontend handling requirement. Is this event handled, ignored, or a spec gap? **Resolution**: FR-016 added to spec.md. Go `PollDeviceFlow` now emits `auth:device-flow-expired` when status == "expired". `useAuth.ts` now subscribes to the event and sets an error message prompting the user to restart the flow. [Ambiguity, Contracts §Events, Gap]

---

## Notes

- Items marked `[Gap]` indicate requirements that are missing from the spec and should be added or explicitly deferred.
- Items marked `[Conflict]` indicate discrepancies between spec, plan, data-model, and contracts docs that need resolution.
- Items marked `[Ambiguity]` indicate requirements that exist but are insufficiently specific for unambiguous implementation.
- All 67 items resolved as of 2026-03-30.
