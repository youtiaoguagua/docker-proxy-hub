package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"docker-proxy-hub/internal/api"
	"docker-proxy-hub/internal/auth"
	"docker-proxy-hub/internal/config"
	"docker-proxy-hub/internal/db"
	"docker-proxy-hub/internal/frontend"
	"docker-proxy-hub/internal/healthchecker"
	"docker-proxy-hub/internal/logger"
	"docker-proxy-hub/internal/proxy"

	"github.com/gin-gonic/gin"
)

// Known frontend SPA routes. All other paths are treated as Docker registry
// image references and forwarded to the proxy handler.
var frontendRoutes = []string{
	"/",
	"/upstreams",
	"/monitoring",
	"/settings",
	"/account",
	"/docs",
	"/login",
	"/setup",
}

func isFrontendPath(path string) bool {
	// Frontend SPA routes
	for _, route := range frontendRoutes {
		if path == route {
			return true
		}
	}
	// Frontend SPA sub-routes (e.g. /docs/usage)
	for _, route := range frontendRoutes {
		if route != "/" && strings.HasPrefix(path, route+"/") {
			return true
		}
	}
	// Frontend index
	if path == "/index.html" {
		return true
	}
	// Static assets
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	// Files with common static extensions
	for _, ext := range []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot", ".map"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	// Favicon
	if path == "/favicon.ico" {
		return true
	}
	return false
}

func main() {
	ctx := context.Background()
	cfg := config.Load()

	logger.Init(cfg.LogLevel, cfg.LogFormat)

	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	slog.Info("starting docker-proxy-hub",
		"addr", cfg.Addr,
		"db", cfg.DBPath,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
		"health_interval", cfg.HealthCheckInterval,
	)

	store, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	authService, err := auth.NewService(ctx, store, cfg.JWTSecret, cfg.JWTTTL)
	if err != nil {
		slog.Error("failed to initialize auth service", "error", err)
		os.Exit(1)
	}

	checker := healthchecker.NewChecker(store, cfg.HealthCheckInterval)
	checker.Start(ctx)

	proxyHandler := proxy.NewHandler(store)

	router := api.NewRouter(api.Dependencies{Auth: authService, Store: store, Checker: checker, Proxy: proxyHandler, CookieName: cfg.CookieName})

	frontendHandler, err := frontend.NewHandler(frontend.DistFS)
	if err != nil {
		slog.Error("failed to load frontend", "error", err)
		os.Exit(1)
	}

	rootHandler := http.NewServeMux()
	rootHandler.Handle("/v2/", proxyHandler)
	rootHandler.Handle("/api/", router)
	rootHandler.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isFrontendPath(r.URL.Path) {
			frontendHandler.ServeHTTP(w, r)
		} else {
			proxyHandler.ServeHTTP(w, r)
		}
	}))

	if err := http.ListenAndServe(cfg.Addr, rootHandler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}