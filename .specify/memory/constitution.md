<!--
SYNC IMPACT REPORT
==================
Version change: 0.0.0 (template) → 1.0.0

Modified principles:
  [PRINCIPLE_1_NAME] → I. Code Quality Standards
  [PRINCIPLE_2_NAME] → II. Testing Standards
  [PRINCIPLE_3_NAME] → III. User Experience Consistency
  [PRINCIPLE_4_NAME] → IV. Performance Requirements
  [PRINCIPLE_5_NAME] → (removed - 4 principles sufficient for this project)

Added sections:
  - Technology Stack (replaces [SECTION_2_NAME])
  - Development Workflow (replaces [SECTION_3_NAME])

Removed sections:
  - None

Templates requiring updates:
  ✅ .specify/templates/plan-template.md — Constitution Check section references
     "Performance Goals" and "Testing" fields already present; no changes needed
  ✅ .specify/templates/spec-template.md — Testing and success criteria sections
     align with Principle II and IV; no structural changes needed
  ✅ .specify/templates/tasks-template.md — Task categories align with principles;
     performance and UX tasks already covered in Polish phase
  ✅ No command files found in .specify/templates/commands/ — no updates needed

Deferred TODOs:
  - RATIFICATION_DATE: set to today (2026-03-30) as first adoption date
-->

# Gitura Constitution

## Core Principles

### I. Code Quality Standards

All Go code MUST adhere to idiomatic Go conventions enforced by `gofmt`, `golint`,
and `go vet`. Every exported symbol MUST have a doc comment. Cyclomatic complexity
per function MUST NOT exceed 10. Code MUST be reviewed by at least one other
contributor before merge. Dead code, commented-out blocks, and TODO comments older
than one sprint MUST be resolved or filed as tracked issues before release.

**Rationale**: GitHub PR review is a precision tool; the codebase itself must model
the quality bar it helps enforce on others.

### II. Testing Standards

- Unit tests MUST accompany every new package and cover all exported functions.
- Minimum line coverage threshold is **80%**; critical review and diff-parsing
  packages MUST reach **90%**.
- Integration tests MUST cover end-to-end GitHub API flows using recorded fixtures
  (no live API calls in CI).
- Tests MUST be written before or alongside implementation (TDD preferred).
- Test names MUST follow `TestFunctionName_Scenario_ExpectedOutcome` convention.
- Flaky tests MUST be quarantined and fixed within one sprint; they MUST NOT be
  merged to main.

**Rationale**: Review tooling that cannot be reliably tested cannot be trusted with
production code review workflows.

### III. User Experience Consistency

- The UI MUST follow a single, documented design language (component library or
  style guide); ad-hoc styling is prohibited.
- All interactive elements (buttons, inputs, diff views) MUST have keyboard
  navigation support and ARIA labels.
- Error states, loading states, and empty states MUST be explicitly handled and
  visually consistent across all views.
- PR list, diff view, and comment thread layouts MUST share a coherent information
  hierarchy — font scales, spacing units, and color tokens MUST NOT diverge between
  screens.
- Any breaking change to a UI component MUST update all consumers in the same PR.

**Rationale**: Developers context-switch rapidly during code review; a fragmented
UI increases cognitive load and reduces review quality.

### IV. Performance Requirements

- Initial application load MUST complete in **≤ 2 seconds** on a standard broadband
  connection (measured via Lighthouse or equivalent).
- PR diff rendering for files up to **5 000 lines** MUST complete in **≤ 500 ms**.
- GitHub API calls MUST be paginated and cached; the same resource MUST NOT be
  fetched more than once per session without explicit user refresh.
- Memory usage of the running process MUST stay below **150 MB** under normal
  single-PR review workloads.
- Performance regressions of **> 10%** on any tracked metric MUST block the merge.

**Rationale**: Review tools that stall or bloat degrade the very workflow they aim
to improve; performance is a first-class feature.

## Technology Stack

- **Language**: Go (latest stable release)
- **UI layer**: TBD — MUST be chosen from Go-native or WebView-based options
  evaluated in Phase 0 research; no mixing of incompatible UI paradigms.
- **GitHub integration**: GitHub REST API v3 and/or GraphQL API v4 via an
  officially supported Go client.
- **Testing**: `go test` with `testify` for assertions; `httptest` for HTTP
  fixture recording.
- **Linting/formatting**: `gofmt`, `golangci-lint` with the project's `.golangci.yml`
  config checked into the repository.
- **Build**: Standard `go build`; no external build systems unless justified and
  documented.

Technology additions or removals MUST be approved via the amendment process and
noted in the Sync Impact Report of the amending constitution version.

## Development Workflow

- All work MUST originate from a Linear issue or GitHub issue; no uncommitted
  speculative changes.
- Feature branches MUST follow the naming convention `###-short-description`
  matching the issue number.
- PRs MUST pass all CI checks (build, lint, tests, coverage gate) before review.
- PRs adding or changing UI components MUST include a screenshot or screen
  recording in the PR description.
- PRs touching the GitHub API integration MUST include updated fixture files if
  the response shape changes.
- The `main` branch MUST always be releasable; broken builds on `main` are a P1
  incident.

## Governance

This constitution supersedes all other practices, verbal agreements, or prior
conventions. When a conflict arises between this document and any other guideline,
this document wins.

**Amendment procedure**:
1. Open a pull request modifying `.specify/memory/constitution.md`.
2. Increment `CONSTITUTION_VERSION` per semantic versioning rules defined herein.
3. Update the Sync Impact Report comment at the top of this file.
4. Obtain approval from at least one other contributor.
5. Propagate changes to dependent templates (plan, spec, tasks) in the same PR.

**Compliance review**: Every PR description MUST include a "Constitution Check"
section confirming no principles are violated, or explicitly documenting and
justifying any approved exception.

**Versioning policy**:
- MAJOR: Principle removed, renamed with changed intent, or governance restructured.
- MINOR: New principle or section added; material expansion of existing guidance.
- PATCH: Wording clarifications, typo fixes, non-semantic refinements.

**Version**: 1.0.0 | **Ratified**: 2026-03-30 | **Last Amended**: 2026-03-30
