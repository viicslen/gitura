package app

import (
	"context"
	"fmt"
	"time"

	"gitura/internal/ghapi"
	"gitura/internal/logger"
	"gitura/internal/model"
)

// branchRefsTimeout bounds the branch lookup so a slow GraphQL answer cannot hold up the list.
const branchRefsTimeout = 5 * time.Second

// ListOpenPRs fetches all open pull requests involving the authenticated user
// using a single GitHub Search query (involves: qualifier). The returned items
// are tagged with IsAuthor, IsAssignee, IsReviewer for client-side filtering.
// Only IncludeDrafts from the filters struct affects the server-side query;
// all other filter fields are applied in the frontend.
// All errors are surfaced via the returned PRListResult.Error field.
func (a *App) ListOpenPRs(filters model.PRListFilters) (model.PRListResult, error) {
	logger.L.Debug("ListOpenPRs called", "include_drafts", filters.IncludeDrafts)

	if a.ghClient == nil {
		logger.L.Warn("ListOpenPRs called without authenticated client")
		return model.PRListResult{Error: "not authenticated"}, nil
	}

	authState, err := a.GetAuthState()
	if err != nil || !authState.IsAuthenticated {
		logger.L.Warn("ListOpenPRs: auth check failed", "err", err)
		return model.PRListResult{Error: "not authenticated"}, nil
	}

	result, err := ghapi.SearchOpenPRs(a.ctx, a.ghClient, authState.Login, filters)
	if err != nil {
		logger.L.Error("SearchOpenPRs error", "err", err)
		return model.PRListResult{Error: fmt.Sprintf("search failed: %v", err)}, nil
	}
	if result.Error == "" {
		// Without branch refs the tree layout shows every PR at the top level; the list itself is still good.
		ctx, cancel := context.WithTimeout(a.ctx, branchRefsTimeout)
		defer cancel()
		if err := ghapi.AddBranchRefs(ctx, ghapi.NewHTTPClient(a.authToken), result.Items); err != nil {
			logger.L.Warn("AddBranchRefs failed", "err", err)
		}
	}

	logger.L.Info("ListOpenPRs complete",
		"items", len(result.Items),
		"incomplete", result.IncompleteResults,
		"error", result.Error,
	)
	return result, nil
}

// SearchPRs runs text as a GitHub pull request search across everything the user can see
// and returns the 20 most recently updated matches. Errors are reported in PRListResult.Error.
func (a *App) SearchPRs(text string) model.PRListResult {
	if a.ghClient == nil {
		return model.PRListResult{Error: "not authenticated"}
	}
	authState, err := a.GetAuthState()
	if err != nil || !authState.IsAuthenticated {
		return model.PRListResult{Error: "not authenticated"}
	}
	return ghapi.SearchPRs(a.ctx, a.ghClient, authState.Login, text)
}
