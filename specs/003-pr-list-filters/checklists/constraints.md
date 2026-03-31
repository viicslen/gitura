# Constraint Completeness & Conflict Detection Checklist: Open PR List with Filters

**Purpose**: Detect missing constraints, under-specified boundaries, and conflicts across all requirement categories
**Created**: 2026-03-31
**Feature**: [spec.md](../spec.md)

## Involvement Type Constraints (FR-001a)

- [x] CHK001 FR-001a states "at least one involvement type must remain active" — the spec does not define what UI mechanism enforces this. **Resolved**: The last active checkbox is disabled in the UI; the toggle action is silently refused in `isOnlyActiveInvolvement`. No error or tooltip is shown.
- [x] CHK002 FR-001a does not specify what happens to the current result set when the last remaining involvement type is about to be deselected. **Resolved**: Same as CHK001 — the action is silently refused; the result set does not change.
- [ ] CHK003 The Assumptions section restates the at-least-one rule, but neither location specifies whether "reviewer" covers review-requested, already-reviewed, or both states — this ambiguity could produce conflicting query logic.

## Date Filter Constraints (FR-006)

- [x] CHK004 FR-006 lists "last 7 days, last 30 days, or a custom range" but "custom range" is never specified. **Resolved**: Custom date range removed from scope — only the date picker (`updated_after`) is implemented. FR-006 updated accordingly.
- [ ] CHK005 FR-006 does not specify what the date field is: "opened or updated" — this is an OR condition across two fields. The spec does not state whether the filter applies to `created_at`, `updated_at`, or either. This creates ambiguity in SC-005 ("0% false positives").
- [ ] CHK006 FR-006 does not define the date reference point for relative ranges ("last 7 days") — is it calendar days, 24-hour periods, or rolling from the current timestamp? This affects reproducibility of SC-003.
- [ ] CHK007 US5 acceptance scenario 1 says "opened or updated within that range" but FR-006 uses the same phrasing without clarifying precedence when a PR was opened outside the range but updated inside it (or vice versa). This creates a potential conflict with SC-005.

## Sort Order vs. Display Age Conflict (FR-002 / FR-010)

- [x] CHK008 FR-002 requires displaying "time elapsed since the PR was opened" (created_at), but FR-010 sorts by "most recently updated first" (updated_at). **Resolved by user**: This is intentional and expected — no spec change needed.
- [ ] CHK009 FR-010 specifies default sort order but does not define whether the user can change the sort order. If sort is always fixed, this is a constraint that should be stated explicitly; if it is user-configurable, there are no requirements covering it.

## Race Condition & Concurrent Query Constraints (FR-013)

- [x] CHK010 FR-013 states results must reflect "the most recent query" but does not specify how staleness from in-flight concurrent requests is handled. **Resolved**: FR-013 updated to require superseding; implemented via `requestId` counter in `usePRFilters.ts`. Each `fetchPRs` call captures `requestId.value` at start; stale responses are discarded on receipt.
- [x] CHK011 The edge case "What if the user applies filters before the initial PR list has finished loading?" has no corresponding functional requirement. **Resolved**: Same `requestId` mechanism supersedes the initial request. Spec edge cases section updated to document this behaviour explicitly.

## AND Logic Conflict with Involvement Types (FR-007 / FR-001a)

- [x] CHK012 FR-007 says multiple filters use AND logic. FR-001a says involvement types are toggled — but involvement types are internally combined with OR. **Resolved by user**: Involvement types are OR-combined; all other filters use AND. FR-007 updated to state this explicitly.
- [x] CHK013 FR-007 does not specify what happens when the repo filter and org filter are both active, and the specified repo does not belong to the specified org. **Resolved**: Zero results, no error; empty-state message indicates active filters. FR-007 updated.

## Session Persistence Constraints (FR-012)

- [ ] CHK014 FR-012 says filter state persists "within the user's session" but "session" is never defined. The Assumptions section says "not persisted across application restarts" — but it is unclear whether navigating to a different page and back within the same running app counts as the same session. This is only partially resolved.
- [ ] CHK015 FR-012 does not specify whether filter state resets when the user logs out and logs back in within the same app process. The boundary between session and auth state is unspecified.

## Rate Limit Constraints (FR-014)

- [x] CHK016 FR-014 says "no automatic retry is performed — the user must manually trigger a refresh." The spec does not define what "manually trigger a refresh" means. **Resolved**: The existing refresh button (RefreshCw) in the PR list toolbar serves as the manual retry control. No additional button needed.
- [ ] CHK017 FR-011 covers generic errors (network failure, expired token) and FR-014 covers rate limits — but the spec does not specify whether an expired token mid-session is handled by FR-011 (graceful error) or requires a distinct behaviour. The two requirements are not clearly disjoint.

## Empty State Differentiation Constraint (FR-009)

- [ ] CHK018 FR-009 requires distinguishing "no PRs exist for this account" from "no PRs match the active filters." The spec does not define how the system determines which state applies when all involvement filters are active but there are no results — it is ambiguous whether that is an "account empty" state or a "filter empty" state.
- [ ] CHK019 FR-009 does not specify what constitutes the "default" state (no filters applied) vs. a "filtered" state for the purpose of empty state messaging. If the user has applied then cleared all filters, which message should appear?

## Performance Constraints (SC-001, SC-002)

- [ ] CHK020 SC-001 and SC-002 both say "within 3 seconds" but do not define the measurement boundary: does the timer start at user action (navigation/filter change), at API request dispatch, or at API response receipt? Without this, the criterion is not independently verifiable.
- [ ] CHK021 SC-002 does not distinguish between network latency and rendering latency. On a slow API response, the 3-second budget may be consumed entirely by the network — no loading state requirement is stated that would make the constraint achievable under real conditions.
- [ ] CHK022 SC-001 references "a standard broadband connection" which is not defined (bandwidth, latency). This makes the success criterion environment-dependent and not reproducibly testable.

## Repository Filter Population Constraint (Assumptions)

- [x] CHK023 The Assumptions section states "repository filter options are populated from the repos that appear in the fetched PR list." **Resolved**: Confirmed as the final approach — repo filter options are derived client-side from the raw `SearchOpenPRs` result (distinct `owner/repo` values). No dedicated API call is needed; the chicken-and-egg concern does not apply because the repo dropdown only appears after load completes.
- [x] CHK024 The assumption that repo options come from the fetched list also conflicts with FR-007 if the repo filter is populated after fetch. **Resolved**: The repo dropdown is populated from `rawResult.items` after load. Filtering is fully client-side so repo options and filter controls are always consistent with the loaded data.

## Pagination and Completeness Constraints (Assumptions)

- [ ] CHK025 The Assumptions section states "pagination is handled automatically to return complete results within the session" but no requirement caps the maximum result set size. With hundreds of open PRs, complete pagination could cause very long load times or hit secondary rate limits — no timeout or truncation requirement is defined.
- [ ] CHK026 SC-003 assumes the user can "reduce a list of 50+ open PRs" but the spec does not define a minimum or maximum supported PR count. If pagination returns thousands of PRs, the 30-second usability target in SC-003 becomes unachievable without additional constraints.

## Draft PR Toggle Constraints (FR-015)

- [ ] CHK027 FR-015 says toggling the "include drafts" toggle triggers a new API query, but does not specify the default toggle state persistence. FR-012 covers general filter state persistence — it is unclear whether the drafts toggle is treated as a filter (and thus persisted per FR-012) or as a display preference with a separate lifecycle.

## Clickability and Navigation Constraint (FR-002)

- [ ] CHK028 FR-002 states each PR entry "MUST be clickable and navigate the user to the in-app PR review/detail view." The spec does not define what happens if the target PR detail view does not yet exist or is unavailable — no fallback behaviour is specified.

## Notes

- Items marked `[ ]` require a spec decision or clarification before the constraint can be considered complete.
- Check items off as resolved: `[x]`, and add an inline note explaining the resolution.
- Conflicts (CHK008, CHK012, CHK013, CHK023) are higher priority as they may require spec edits, not just additions.
- Items CHK004, CHK010, CHK011 directly affect testability of acceptance scenarios.
