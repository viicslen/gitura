package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gitura/internal/model"
)

// githubGraphQLURL is the GitHub GraphQL API endpoint.
// It is a package-level variable to allow overriding in tests.
var githubGraphQLURL = "https://api.github.com/graphql"

// graphQLReviewThreadsResponse is the top-level GraphQL response wrapper.
type graphQLReviewThreadsResponse struct {
	Data   graphQLReviewData `json:"data"`
	Errors []graphQLError    `json:"errors"`
}

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
	Comments   graphQLCommentConnection `json:"comments"`
}

type graphQLCommentConnection struct {
	Nodes []graphQLComment `json:"nodes"`
}

type graphQLComment struct {
	DatabaseID   int64           `json:"databaseId"`
	Body         string          `json:"body"`
	Author       graphQLActor    `json:"author"`
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

type graphQLError struct {
	Message string `json:"message"`
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

		payload := map[string]interface{}{
			"query":     reviewThreadsQueryBase,
			"variables": vars,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("github: marshal GraphQL request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubGraphQLURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("github: create GraphQL request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github: GraphQL request failed: %w", err)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("auth: GitHub API returned 401 — token invalid or missing 'repo' scope")
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("github: GraphQL returned HTTP %d", resp.StatusCode)
		}

		var gqlResp graphQLReviewThreadsResponse
		if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("github: decode GraphQL response: %w", err)
		}
		_ = resp.Body.Close()

		if len(gqlResp.Errors) > 0 {
			msgs := make([]string, len(gqlResp.Errors))
			for i, e := range gqlResp.Errors {
				msgs[i] = e.Message
			}
			return nil, fmt.Errorf("github: GraphQL errors: %s", strings.Join(msgs, "; "))
		}

		nodes := gqlResp.Data.Repository.PullRequest.ReviewThreads.Nodes
		for _, node := range nodes {
			thread := mapGraphQLThread(node)
			threads = append(threads, thread)
		}

		if progressFn != nil {
			progressFn(len(threads), -1)
		}

		pageInfo := gqlResp.Data.Repository.PullRequest.ReviewThreads.PageInfo
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
			Comments: []model.CommentDTO{},
		}
	}

	root := node.Comments.Nodes[0]
	line := 0
	if root.Line != nil {
		line = *root.Line
	} else if root.OriginalLine != nil {
		line = *root.OriginalLine
	}

	comments := make([]model.CommentDTO, 0, len(node.Comments.Nodes))
	for _, c := range node.Comments.Nodes {
		var replyToID int64
		if c.ReplyTo != nil {
			replyToID = c.ReplyTo.DatabaseID
		}
		comments = append(comments, model.CommentDTO{
			ID:           c.DatabaseID,
			InReplyToID:  replyToID,
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
		NodeID:   node.ID,
		Comments: comments,
		Resolved: node.IsResolved,
		Path:     root.Path,
		Line:     line,
	}
}
