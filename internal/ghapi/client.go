// Package ghapi wraps the GitHub REST and GraphQL APIs the app uses and maps
// their responses to model DTOs. Errors are prefixed with "github:" (or "auth:" on 401).
package ghapi

import (
	"context"
	"net/http"

	"github.com/google/go-github/v93/github"
	"golang.org/x/oauth2"
)

// NewClient returns an authenticated go-github client using the supplied
// personal access token or OAuth token.
func NewClient(token string) (*github.Client, error) {
	return github.NewClient(github.WithHTTPClient(NewHTTPClient(token)))
}

// NewHTTPClient returns a plain *http.Client authenticated with the given token.
// Useful for raw HTTP calls such as GitHub GraphQL requests.
func NewHTTPClient(token string) *http.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return oauth2.NewClient(context.Background(), ts)
}
