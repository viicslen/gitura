// Package github provides a factory for creating authenticated GitHub API clients.
package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v67/github"
	"golang.org/x/oauth2"
)

// NewClient returns an authenticated go-github client using the supplied
// personal access token or OAuth token.
func NewClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}

// NewHTTPClient returns a plain *http.Client authenticated with the given token.
// Useful for raw HTTP calls such as GitHub GraphQL requests.
func NewHTTPClient(token string) *http.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return oauth2.NewClient(context.Background(), ts)
}
