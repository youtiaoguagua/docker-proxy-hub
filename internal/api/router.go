package api

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"docker-proxy-hub/internal/auth"
	"docker-proxy-hub/internal/db"
	"docker-proxy-hub/internal/proxy"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Auth       *auth.Service
	Store      *db.Store
	Checker    checker
	Proxy      *proxy.Handler
	CookieName string
}

type checker interface {
	GetAllHealth() []proxy.Upstream
	ForceCheckAll(ctx context.Context)
	ForceCheckOne(ctx context.Context, upstreamID int64)
	SpeedTestOne(ctx context.Context, upstreamID int64) (proxy.Upstream, error)
}

type Server struct {
	deps Dependencies
}

func NewRouter(deps Dependencies) *gin.Engine {
	server := &Server{deps: deps}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	// Public routes
	r.GET("/api/health", server.health)
	r.GET("/api/setup/status", server.setupStatus)
	r.POST("/api/setup", server.setup)
	r.POST("/api/auth/login", server.login)

	// Auth-required routes
	authGroup := r.Group("", server.requireAuth())
	authGroup.POST("/api/auth/logout", server.logout)
	authGroup.GET("/api/auth/me", server.me)
	authGroup.PUT("/api/auth/admin", server.changeAdmin)
	authGroup.GET("/api/dashboard/summary", server.dashboardSummary)
	authGroup.GET("/api/upstreams", server.listUpstreams)
	authGroup.POST("/api/upstreams", server.createUpstream)
	authGroup.PUT("/api/upstreams/:id", server.updateUpstream)
	authGroup.DELETE("/api/upstreams/:id", server.deleteUpstream)
	authGroup.POST("/api/upstreams/check-health", server.checkHealth)
	authGroup.POST("/api/upstreams/:id/check-health", server.checkHealthOne)
	authGroup.POST("/api/upstreams/:id/speed-test", server.speedTest)
	authGroup.GET("/api/monitoring/health", server.monitoringHealth)
	authGroup.GET("/api/monitoring/logs", server.monitoringLogs)

	return r
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) setupStatus(c *gin.Context) {
	required, err := s.deps.Auth.SetupRequired(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to read setup status")
		return
	}
	c.JSON(http.StatusOK, gin.H{"setupRequired": required})
}

func (s *Server) setup(c *gin.Context) {
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	admin, token, err := s.deps.Auth.Setup(c.Request.Context(), request.Username, request.Password)
	if err != nil {
		s.writeAuthError(c, err)
		return
	}
	s.setCookie(c, token)
	c.JSON(http.StatusCreated, gin.H{"admin": admin})
}

func (s *Server) login(c *gin.Context) {
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	admin, token, err := s.deps.Auth.Login(c.Request.Context(), request.Username, request.Password)
	if err != nil {
		s.writeAuthError(c, err)
		return
	}
	s.setCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"admin": admin})
}

func (s *Server) logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: s.deps.CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"admin": currentAdmin(c)})
}

func (s *Server) changeAdmin(c *gin.Context) {
	var request changeAdminRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	admin, err := s.deps.Auth.ChangeCredentials(c.Request.Context(), currentAdmin(c).ID, request.CurrentPassword, request.Username, request.Password)
	if err != nil {
		s.writeAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"admin": admin})
}

func (s *Server) dashboardSummary(c *gin.Context) {
	summary, err := s.deps.Store.DashboardSummary(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to load dashboard summary")
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (s *Server) listUpstreams(c *gin.Context) {
	upstreams, err := s.deps.Store.ListUpstreams(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to list upstreams")
		return
	}
	if upstreams == nil {
		upstreams = []proxy.Upstream{}
	}
	c.JSON(http.StatusOK, gin.H{"upstreams": upstreams})
}

func (s *Server) createUpstream(c *gin.Context) {
	var input proxy.UpstreamInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	upstream, err := s.deps.Store.CreateUpstream(c.Request.Context(), input)
	if err != nil {
		writeError(c, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"upstream": upstream})
}

func (s *Server) updateUpstream(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var input proxy.UpstreamInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	upstream, err := s.deps.Store.UpdateUpstream(c.Request.Context(), id, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(c, http.StatusNotFound, "not_found", "upstream not found")
			return
		}
		writeError(c, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"upstream": upstream})
}

func (s *Server) deleteUpstream(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := s.deps.Store.DeleteUpstream(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(c, http.StatusNotFound, "not_found", "upstream not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to delete upstream")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) checkHealth(c *gin.Context) {
	s.deps.Checker.ForceCheckAll(c.Request.Context())
	upstreams, err := s.deps.Store.ListUpstreams(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to list upstreams")
		return
	}
	if upstreams == nil {
		upstreams = []proxy.Upstream{}
	}
	c.JSON(http.StatusOK, gin.H{"upstreams": upstreams})
}

func (s *Server) checkHealthOne(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	s.deps.Checker.ForceCheckOne(c.Request.Context(), id)
	upstream, err := s.deps.Store.GetUpstream(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "upstream not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"upstream": upstream})
}

func (s *Server) speedTest(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	upstream, err := s.deps.Checker.SpeedTestOne(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusBadRequest, "speed_test_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"upstream": upstream})
}

func (s *Server) monitoringHealth(c *gin.Context) {
	healthResults := s.deps.Checker.GetAllHealth()
	if healthResults == nil {
		healthResults = []proxy.Upstream{}
	}
	upstreams, err := s.deps.Store.ListUpstreams(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to load upstreams")
		return
	}
	summary, err := s.deps.Store.DashboardSummary(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to load summary")
		return
	}
	if upstreams == nil {
		upstreams = []proxy.Upstream{}
	}
	c.JSON(http.StatusOK, gin.H{
		"upstreams": upstreams,
		"health":    healthResults,
		"summary":   summary,
	})
}

func (s *Server) monitoringLogs(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	logs, err := s.deps.Store.ListRequestLogs(c.Request.Context(), limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "failed to load request logs")
		return
	}
	if logs == nil {
		logs = []db.RequestLog{}
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(s.deps.CookieName)
		if err != nil || strings.TrimSpace(token) == "" {
			writeError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
			c.Abort()
			return
		}
		admin, err := s.deps.Auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			writeError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
			c.Abort()
			return
		}
		c.Set("admin", admin)
		c.Next()
	}
}

func currentAdmin(c *gin.Context) auth.Admin {
	val, _ := c.Get("admin")
	admin, _ := val.(auth.Admin)
	return admin
}

func (s *Server) setCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{Name: s.deps.CookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(24 * time.Hour)})
}

func (s *Server) writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrSetupExists):
		writeError(c, http.StatusConflict, "setup_exists", "admin already exists")
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(c, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
	case errors.Is(err, auth.ErrValidation):
		writeError(c, http.StatusBadRequest, "validation_error", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "request failed")
	}
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changeAdminRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	CurrentPassword string `json:"currentPassword"`
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "validation_error", "invalid id")
		return 0, false
	}
	return id, true
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		slog.Info("api request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}