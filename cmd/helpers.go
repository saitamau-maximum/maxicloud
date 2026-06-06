package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

type connectRoute struct {
	path    string
	handler http.Handler
}

func route(path string, h http.Handler) connectRoute {
	return connectRoute{path: path, handler: h}
}

func mountAll(r chi.Router, routes ...connectRoute) {
	for _, route := range routes {
		r.Mount(route.path, route.handler)
	}
}

func postgresDSN(user, password, host string, port int, database string) string {
	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s",
		user,
		password,
		host,
		port,
		database,
	)
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func allowedOrigins(redirects []string) []string {
	origins := make([]string, 0, len(redirects))
	seen := map[string]struct{}{}
	for _, redirect := range redirects {
		u, err := url.Parse(redirect)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		origin := u.Scheme + "://" + u.Host
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins
}
