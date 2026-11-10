package ghapi

import (
	"context"
	"net/http"
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
	ID         string                   `json:"id"`
	IsResolved bool                     `json:"isResolved"`
	DiffSide   string                   `json:"diffSide"`
	Comments   graphQLCommentConnection `json:"comments"`
}

type graphQLCommentConnection struct {
	Nodes []graphQLComment `json:"nodes"`
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
          diffSide
          comments(first: 100) {
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
	var (
		threads     []model.CommentThreadDTO
		afterCursor *string
	)

	for {
		vars := map[string]interface{}{
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
			return nil, err
		}

		nodes := data.Repository.PullRequest.ReviewThreads.Nodes
		for _, node := range nodes {
			thread := mapGraphQLThread(node)
			threads = append(threads, thread)
		}

		if progressFn != nil {
			progressFn(len(threads), -1)
		}

		pageInfo := data.Repository.PullRequest.ReviewThreads.PageInfo
		if !pageInfo.HasNextPage {
			break
		}
		afterCursor = &pageInfo.EndCursor
	}

	if threads == nil {
		threads = []model.CommentThreadDTO{}
	}
	return threads, nil
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
	outdated := false
	if root.Line != nil {
		line = *root.Line
	} else if root.OriginalLine != nil {
		line = *root.OriginalLine
		outdated = true
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
