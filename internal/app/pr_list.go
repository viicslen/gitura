package app

import (
	"fmt"

	"gitura/internal/ghapi"
	"gitura/internal/logger"
	"gitura/internal/model"
)

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

	logger.L.Info("ListOpenPRs complete",
		"items", len(result.Items),
		"incomplete", result.IncompleteResults,
		"error", result.Error,
	)
	return result, nil
}
