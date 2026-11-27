package ghapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"gitura/internal/model"
)

type graphQLReviewData struct {
	Repository graphQLRepository `json:"repository"`
}

type graphQLRepository struct {
	PullRequest graphQLPullRequest `json:"pullRequest"`
}

type graphQLPullRequest struct {
	ReviewThreads graphQLThreadConnection `json:"reviewThreads"`
}

type graphQLThreadConnection struct {
	PageInfo graphQLPageInfo `json:"pageInfo"`
	Nodes    []graphQLThread `json:"nodes"`
}

type graphQLPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type graphQLThread struct {
	ID            string                   `json:"id"`
	IsResolved    bool                     `json:"isResolved"`
	IsOutdated    bool                     `json:"isOutdated"`
	DiffSide      string                   `json:"diffSide"`
	OriginalLine  *int                     `json:"originalLine"`
	OriginalStart *int                     `json:"originalStartLine"`
	StartDiffSide string                   `json:"startDiffSide"`
	SubjectType   string                   `json:"subjectType"`
	Comments      graphQLCommentConnection `json:"comments"`
}

type graphQLCommentConnection struct {
	PageInfo graphQLPageInfo  `json:"pageInfo"`
	Nodes    []graphQLComment `json:"nodes"`
}

type graphQLComment struct {
	DatabaseID   int64           `json:"databaseId"`
	Body         string          `json:"body"`
	Author       graphQLActor    `json:"author"`
	Review       *graphQLReview  `json:"pullRequestReview"`
	Path         string          `json:"path"`
	Line         *int            `json:"line"`
	OriginalLine *int            `json:"originalLine"`
	DiffHunk     string          `json:"diffHunk"`
	CreatedAt    string          `json:"createdAt"` // ISO-8601
	URL          string          `json:"url"`
	ReplyTo      *graphQLReplyTo `json:"replyTo"`
}

type graphQLActor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl"`
}

type graphQLReplyTo struct {
	DatabaseID int64 `json:"databaseId"`
}

type graphQLReview struct {
	DatabaseID int64 `json:"databaseId"`
}

// reviewThreadsQuery is the GraphQL query for paginated review threads.
// The after argument is omitted on the first page and set to the cursor thereafter.
const reviewThreadsQueryBase = `
query($owner: String!, $repo: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 50, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          isOutdated
          diffSide
          originalLine
          originalStartLine
          startDiffSide
          subjectType
          comments(first: 100) {
            pageInfo { hasNextPage endCursor }
            nodes {
              databaseId
              body
              author { login avatarUrl }
              pullRequestReview { databaseId }
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
`

// FetchReviewThreads fetches all review threads for a pull request using the
// GitHub GraphQL API. Results are paginated (50 threads per page).
//
// The progressFn callback is called after each page with (loaded, total) thread
// counts. total is -1 until all pages are fetched (GitHub doesn't return a total
// count in the GraphQL API).
//
// Returns a slice of CommentThreadDTO on success.
// Errors are prefixed with "github:" on API failures or "auth:" on HTTP 401.
func FetchReviewThreads(
	ctx context.Context,
	httpClient *http.Client,
	owner, repo string,
	number int,
	progressFn func(loaded, total int),
) ([]model.CommentThreadDTO, error) {
	threads := []model.CommentThreadDTO{}
	err := forEachThreadPage(ctx, httpClient, owner, repo, number, func(nodes []graphQLThread) {
		for _, node := range nodes {
			threads = append(threads, mapGraphQLThread(node))
		}
		if progressFn != nil {
			progressFn(len(threads), -1)
		}
	})
	if err != nil {
		return nil, err
	}
	return threads, nil
}

// ErrUnheldComments is a pending review holding comments a draft can't: replies to existing threads.
var ErrUnheldComments = errors.New("conflict: your pending review on GitHub has replies gitura can't edit; submit or discard it on GitHub")

// FetchPendingReviewDrafts returns the threads that the pending review reviewID starts, as drafts.
// The REST review-comments endpoint omits line and side on pending comments, so threads are read instead.
// Errors are prefixed with "github:" on API failures, "auth:" on HTTP 401, and "conflict:" when the
// review holds comments a draft can't (ErrUnheldComments), since submitting would drop them.
func FetchPendingReviewDrafts(
	ctx context.Context,
	httpClient *http.Client,
	owner, repo string,
	number int,
	reviewID int64,
) ([]model.DraftCommentDTO, error) {
	var drafts []model.DraftCommentDTO
	unheld := 0
	err := forEachThreadPage(ctx, httpClient, owner, repo, number, func(nodes []graphQLThread) {
		for _, node := range nodes {
			d, ok := pendingDraft(node, reviewID)
			if !ok {
				unheld++
			} else if d != nil {
				drafts = append(drafts, *d)
			}
		}
	})
	if err != nil {
		return nil, err
	}
	if unheld > 0 {
		return nil, ErrUnheldComments
	}
	return drafts, nil
}

// pendingDraft returns the draft for a thread that reviewID starts, or nil for a thread it doesn't.
// ok is false when the review has a comment in the thread that a draft can't hold.
func pendingDraft(node graphQLThread, reviewID int64) (*model.DraftCommentDTO, bool) {
	inReview := func(c graphQLComment) bool { return c.Review != nil && c.Review.DatabaseID == reviewID }
	if node.Comments.PageInfo.HasNextPage {
		// The review may hold a reply past the first page, which submitting would drop.
		return nil, false
	}
	if len(node.Comments.Nodes) == 0 {
		return nil, true
	}
	if slices.ContainsFunc(node.Comments.Nodes[1:], inReview) {
		return nil, false
	}
	root := node.Comments.Nodes[0]
	if !inReview(root) {
		return nil, true
	}
	d := &model.DraftCommentDTO{Path: root.Path, Body: root.Body, Side: node.DiffSide, DiffHunk: root.DiffHunk}
	// GitHub reports line 1 on a file thread.
	if node.SubjectType == "FILE" {
		return d, true
	}
	if node.OriginalLine == nil {
		return nil, false
	}
	d.Line = *node.OriginalLine
	if node.OriginalStart != nil {
		d.StartLine, d.StartSide = *node.OriginalStart, node.StartDiffSide
	}
	return d, true
}

// forEachThreadPage passes each page of the PR's review threads to fn, 50 threads at a time.
func forEachThreadPage(
	ctx context.Context,
	httpClient *http.Client,
	owner, repo string,
	number int,
	fn func([]graphQLThread),
) error {
	var afterCursor *string
	for {
		vars := map[string]any{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"after":  nil,
		}
		if afterCursor != nil {
			vars["after"] = *afterCursor
		}

		var data graphQLReviewData
		if err := runGraphQL(ctx, httpClient, reviewThreadsQueryBase, vars, &data); err != nil {
			return err
		}
		fn(data.Repository.PullRequest.ReviewThreads.Nodes)

		pageInfo := data.Repository.PullRequest.ReviewThreads.PageInfo
		if !pageInfo.HasNextPage {
			return nil
		}
		afterCursor = &pageInfo.EndCursor
	}
}

// mapGraphQLThread converts a GraphQL thread node to a CommentThreadDTO.
func mapGraphQLThread(node graphQLThread) model.CommentThreadDTO {
	if len(node.Comments.Nodes) == 0 {
		return model.CommentThreadDTO{
			NodeID:   node.ID,
			Resolved: node.IsResolved,
			Comments: []model.ReviewCommentDTO{},
		}
	}

	root := node.Comments.Nodes[0]
	line := 0
	outdated := node.IsOutdated
	if root.Line != nil {
		line = *root.Line
	} else if root.OriginalLine != nil {
		line = *root.OriginalLine
		outdated = true
	}

	if node.SubjectType == "FILE" {
		line, outdated = 0, node.IsOutdated
	}

	side := node.DiffSide
	if side == "" {
		side = "RIGHT"
	}

	comments := make([]model.ReviewCommentDTO, 0, len(node.Comments.Nodes))
	for _, c := range node.Comments.Nodes {
		var replyToID int64
		var reviewID int64
		if c.ReplyTo != nil {
			replyToID = c.ReplyTo.DatabaseID
		}
		if c.Review != nil {
			reviewID = c.Review.DatabaseID
		}
		comments = append(comments, model.ReviewCommentDTO{
			ID:           c.DatabaseID,
			InReplyToID:  replyToID,
			ReviewID:     reviewID,
			Body:         c.Body,
			AuthorLogin:  c.Author.Login,
			AuthorAvatar: c.Author.AvatarURL,
			DiffHunk:     c.DiffHunk,
			CreatedAt:    c.CreatedAt,
			IsSuggestion: strings.Contains(c.Body, "```suggestion"),
		})
	}

	return model.CommentThreadDTO{
		RootID:   root.DatabaseID,
		ReviewID: comments[0].ReviewID,
		NodeID:   node.ID,
		Comments: comments,
		Resolved: node.IsResolved,
		Path:     root.Path,
		Line:     line,
		Side:     side,
		Outdated: outdated,
	}
}
