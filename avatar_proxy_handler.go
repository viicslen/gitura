package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var avatarHTTPClient = &http.Client{Timeout: 10 * time.Second}

func newAvatarProxyHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/__avatar", handleAvatarProxy)
	return mux
}

func avatarProxyMiddleware(next http.Handler) http.Handler {
	handler := newAvatarProxyHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__avatar" {
			handler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
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
