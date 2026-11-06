package main

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v67/github"

	"gitura/internal/logger"
)

var avatarHTTPClient = &http.Client{Timeout: 10 * time.Second}

func newAssetProxyHandler(app *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/__avatar", handleAvatarProxy)
	mux.HandleFunc("/__markdown-image", app.handleMarkdownImageProxy)
	return mux
}

func assetProxyMiddleware(app *App) func(http.Handler) http.Handler {
	handler := newAssetProxyHandler(app)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/__avatar" || r.URL.Path == "/__markdown-image" {
				handler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func handleAvatarProxy(w http.ResponseWriter, r *http.Request) {
	u, ok := parseAvatarRequestURL(r)
	if !ok {
		http.Error(w, "invalid avatar request", http.StatusBadRequest)
		return
	}

	resp, ok := fetchAvatarResponse(w, r, u)
	if !ok {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	copyAvatarHeaders(w, resp)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		http.Error(w, "failed to stream avatar", http.StatusBadGateway)
	}
}

func (a *App) handleMarkdownImageProxy(w http.ResponseWriter, r *http.Request) {
	if u, ok := parseMarkdownImageRequestURL(r); ok {
		logger.L.Debug("markdown image proxy remote", "url", u.String())
		streamRemoteMarkdownImage(w, r, u)
		return
	}

	owner, repo, ref, path, ok := parseMarkdownImageRepoRequest(r)
	if !ok {
		http.Error(w, "invalid markdown image request", http.StatusBadRequest)
		return
	}
	if a.ghClient == nil {
		http.Error(w, "github client unavailable", http.StatusServiceUnavailable)
		return
	}
	logger.L.Debug("markdown image proxy repo", "owner", owner, "repo", repo, "ref", ref, "path", path)
	streamRepositoryMarkdownImage(w, r, a.ghClient, owner, repo, ref, path)
}

func streamRemoteMarkdownImage(w http.ResponseWriter, r *http.Request, u *url.URL) {
	resp, ok := fetchAvatarResponse(w, r, u)
	if !ok {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	copyAvatarHeaders(w, resp)
	streamMarkdownImageBody(w, resp.Body)
}

func streamRepositoryMarkdownImage(w http.ResponseWriter, r *http.Request, client *github.Client, owner, repo, ref, path string) {
	if streamRepositoryMarkdownImageRawAPI(w, r, client, owner, repo, ref, path) {
		return
	}

	if streamRepositoryMarkdownImageContent(w, r, client, owner, repo, ref, path) {
		return
	}

	streamRepositoryMarkdownImageDownload(w, r, client, owner, repo, ref, path)
}

func streamRepositoryMarkdownImageRawAPI(w http.ResponseWriter, r *http.Request, client *github.Client, owner, repo, ref, path string) bool {
	rawPath := githubContentsRawPath(owner, repo, path, ref)
	req, err := client.NewRequest(http.MethodGet, rawPath, nil)
	if err != nil {
		logger.L.Warn("markdown image raw api request build failed", "err", err, "request_path", rawPath)
		return false
	}
	req = req.WithContext(r.Context())
	req.Header.Set("Accept", "application/vnd.github.raw")

	resp, err := client.Client().Do(req)
	if err != nil {
		logger.L.Warn("markdown image raw api request failed", "err", err, "request_path", rawPath)
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		logger.L.Warn("markdown image raw api non-200", "status", resp.StatusCode, "request_path", rawPath)
		return false
	}

	copyAvatarHeaders(w, resp)
	setMarkdownImageContentType(w, path)
	streamMarkdownImageBody(w, resp.Body)
	return true
}

func streamRepositoryMarkdownImageContent(w http.ResponseWriter, r *http.Request, client *github.Client, owner, repo, ref, path string) bool {
	fileContent, _, resp, err := client.Repositories.GetContents(
		r.Context(),
		owner,
		repo,
		path,
		&github.RepositoryContentGetOptions{Ref: ref},
	)
	if err != nil || fileContent == nil || resp == nil || resp.StatusCode != http.StatusOK {
		logger.L.Warn(
			"markdown image get contents failed",
			"err", err,
			"status", githubStatusCode(resp),
			"owner", owner,
			"repo", repo,
			"ref", ref,
			"path", path,
		)
		return false
	}

	content, err := fileContent.GetContent()
	if err != nil {
		logger.L.Warn("markdown image content decode failed", "err", err, "owner", owner, "repo", repo, "ref", ref, "path", path)
		return false
	}

	copyAvatarHeaders(w, resp.Response)
	setMarkdownImageContentType(w, path)
	streamMarkdownImageBody(w, strings.NewReader(content))
	return true
}

func streamRepositoryMarkdownImageDownload(w http.ResponseWriter, r *http.Request, client *github.Client, owner, repo, ref, path string) {
	reader, resp, err := client.Repositories.DownloadContents(
		r.Context(),
		owner,
		repo,
		path,
		&github.RepositoryContentGetOptions{Ref: ref},
	)
	if err != nil {
		logger.L.Warn("markdown image download contents failed", "err", err, "owner", owner, "repo", repo, "ref", ref, "path", path)
		http.Error(w, "failed to fetch markdown image", http.StatusBadGateway)
		return
	}
	defer func() { _ = reader.Close() }()
	if resp == nil || resp.StatusCode != http.StatusOK {
		logger.L.Warn("markdown image download contents non-200", "status", githubStatusCode(resp), "owner", owner, "repo", repo, "ref", ref, "path", path)
		http.Error(w, "upstream markdown image request failed", http.StatusBadGateway)
		return
	}

	copyAvatarHeaders(w, resp.Response)
	setMarkdownImageContentType(w, path)
	streamMarkdownImageBody(w, reader)
}

func setMarkdownImageContentType(w http.ResponseWriter, path string) {
	if w.Header().Get("Content-Type") != "" {
		return
	}
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
}

func streamMarkdownImageBody(w http.ResponseWriter, body io.Reader) {
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		http.Error(w, "failed to stream markdown image", http.StatusBadGateway)
	}
}

func parseAvatarRequestURL(r *http.Request) (*url.URL, bool) {
	if r.Method != http.MethodGet {
		return nil, false
	}

	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" {
		return nil, false
	}

	u, err := url.Parse(raw)
	if err != nil || !isAllowedAvatarURL(u) {
		return nil, false
	}
	return u, true
}

func parseMarkdownImageRequestURL(r *http.Request) (*url.URL, bool) {
	if r.Method != http.MethodGet {
		return nil, false
	}

	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" {
		return nil, false
	}

	u, err := url.Parse(raw)
	if err != nil || !isAllowedMarkdownImageURL(u) {
		return nil, false
	}
	return u, true
}

func parseMarkdownImageRepoRequest(r *http.Request) (string, string, string, string, bool) {
	if r.Method != http.MethodGet {
		return "", "", "", "", false
	}

	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	path, ok := sanitizeMarkdownImagePath(rawPath)
	if owner == "" || repo == "" || ref == "" || !ok {
		return "", "", "", "", false
	}
	return owner, repo, ref, path, true
}

func sanitizeMarkdownImagePath(raw string) (string, bool) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(raw, "/"))
	if trimmed == "" || strings.Contains(trimmed, `\`) {
		return "", false
	}
	cleaned := pathpkg.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

func githubContentsRawPath(owner, repo, path, ref string) string {
	escapedSegments := strings.Split(path, "/")
	for i := range escapedSegments {
		escapedSegments[i] = url.PathEscape(escapedSegments[i])
	}
	return fmt.Sprintf(
		"repos/%s/%s/contents/%s?ref=%s",
		url.PathEscape(owner),
		url.PathEscape(repo),
		strings.Join(escapedSegments, "/"),
		url.QueryEscape(ref),
	)
}

func githubStatusCode(resp *github.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func fetchAvatarResponse(w http.ResponseWriter, r *http.Request, u *url.URL) (*http.Response, bool) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return nil, false
	}
	req.Header.Set("User-Agent", "gitura")
	req.Header.Set("Accept", "image/*")

	resp, err := avatarHTTPClient.Do(req)
	if err != nil {
		http.Error(w, "failed to fetch avatar", http.StatusBadGateway)
		return nil, false
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		http.Error(w, "upstream avatar request failed", http.StatusBadGateway)
		return nil, false
	}
	return resp, true
}

func copyAvatarHeaders(w http.ResponseWriter, resp *http.Response) {
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
}

func isAllowedAvatarURL(u *url.URL) bool {
	if u == nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "avatars.githubusercontent.com" {
		return true
	}
	return strings.HasSuffix(host, ".githubusercontent.com")
}

func isAllowedMarkdownImageURL(u *url.URL) bool {
	if u == nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "github.com" || host == "raw.githubusercontent.com" || host == "media.githubusercontent.com" || host == "storage.googleapis.com" {
		return true
	}
	return strings.HasSuffix(host, ".githubusercontent.com")
}
