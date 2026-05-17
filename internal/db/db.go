package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"docker-proxy-hub/internal/auth"
	"docker-proxy-hub/internal/health"
	"docker-proxy-hub/internal/metrics"
	"docker-proxy-hub/internal/proxy"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	store := &Store{db: database}
	if err := store.migrate(ctx); err != nil {
		_ = database.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS admins (id INTEGER PRIMARY KEY CHECK (id = 1), username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, token_version INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS upstreams (id INTEGER PRIMARY KEY AUTOINCREMENT, registry_prefix TEXT NOT NULL, name TEXT NOT NULL, base_url TEXT NOT NULL, priority INTEGER NOT NULL DEFAULT 100, enabled INTEGER NOT NULL DEFAULT 1, health_enabled INTEGER NOT NULL DEFAULT 1, health_path TEXT NOT NULL DEFAULT '/v2/', http_proxy TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS upstream_health (upstream_id INTEGER PRIMARY KEY, status TEXT NOT NULL, latency_ms INTEGER NULL, status_code INTEGER NULL, error TEXT NULL, checked_at TEXT NULL)`,
		`CREATE TABLE IF NOT EXISTS request_metrics (id INTEGER PRIMARY KEY AUTOINCREMENT, registry_prefix TEXT NOT NULL, upstream_id INTEGER NULL, method TEXT NOT NULL, path TEXT NOT NULL, status_code INTEGER NOT NULL, duration_ms INTEGER NOT NULL, bytes INTEGER NULL, error TEXT NULL, failover INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	// Add columns for upgrades from previous schemas
	s.db.ExecContext(ctx, `ALTER TABLE upstreams ADD COLUMN http_proxy TEXT NOT NULL DEFAULT ''`)
	s.db.ExecContext(ctx, `ALTER TABLE upstreams ADD COLUMN speed_test_image TEXT NOT NULL DEFAULT ''`)
	s.db.ExecContext(ctx, `ALTER TABLE upstream_health ADD COLUMN download_speed_kbps REAL`)
	s.db.ExecContext(ctx, `ALTER TABLE upstream_health ADD COLUMN manifest_time_ms INTEGER`)
	s.db.ExecContext(ctx, `ALTER TABLE upstream_health ADD COLUMN speed_test_at TEXT`)
	return nil
}

func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) CreateAdmin(ctx context.Context, username, passwordHash string) (auth.Admin, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO admins (id, username, password_hash, token_version, created_at, updated_at) VALUES (1, ?, ?, 1, ?, ?)`, username, passwordHash, now, now)
	if err != nil {
		return auth.Admin{}, err
	}
	return s.GetAdminByID(ctx, 1)
}

func (s *Store) GetAdminByUsername(ctx context.Context, username string) (auth.Admin, error) {
	return s.scanAdmin(s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, token_version FROM admins WHERE username = ?`, username))
}

func (s *Store) GetAdminByID(ctx context.Context, id int64) (auth.Admin, error) {
	return s.scanAdmin(s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, token_version FROM admins WHERE id = ?`, id))
}

func (s *Store) UpdateAdmin(ctx context.Context, id int64, username, passwordHash string) (auth.Admin, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE admins SET username = ?, password_hash = ?, token_version = token_version + 1, updated_at = ? WHERE id = ?`, username, passwordHash, now, id)
	if err != nil {
		return auth.Admin{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return auth.Admin{}, err
	}
	if rows == 0 {
		return auth.Admin{}, sql.ErrNoRows
	}
	return s.GetAdminByID(ctx, id)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanAdmin(row rowScanner) (auth.Admin, error) {
	var admin auth.Admin
	if err := row.Scan(&admin.ID, &admin.Username, &admin.PasswordHash, &admin.TokenVersion); err != nil {
		return auth.Admin{}, err
	}
	return admin, nil
}

func (s *Store) GetOrCreateSetting(ctx context.Context, key string, generator func() (string, error)) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	value, err = generator()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO app_settings (key, value, created_at, updated_at) VALUES (?, ?, ?, ?)`, key, value, now, now)
	return value, err
}

const upstreamCols = `u.id, u.registry_prefix, u.name, u.base_url, u.priority, u.enabled, u.health_enabled, u.health_path, u.http_proxy, u.speed_test_image, COALESCE(h.status, 'unknown'), COALESCE(h.latency_ms, 0), COALESCE(h.status_code, 0), COALESCE(h.error, ''), COALESCE(h.checked_at, ''), COALESCE(h.download_speed_kbps, 0), COALESCE(h.manifest_time_ms, 0)`

const upstreamJoin = `upstreams u LEFT JOIN upstream_health h ON u.id = h.upstream_id`

func scanUpstreamRow(row rowScanner) (proxy.Upstream, error) {
	var upstream proxy.Upstream
	var enabled, healthEnabled int
	var healthStatus string
	var latencyMs, statusCode, healthErr, checkedAt string
	var downloadSpeedKbps, manifestTimeMs string
	if err := row.Scan(&upstream.ID, &upstream.RegistryPrefix, &upstream.Name, &upstream.BaseURL, &upstream.Priority, &enabled, &healthEnabled, &upstream.HealthPath, &upstream.HttpProxy, &upstream.SpeedTestImage, &healthStatus, &latencyMs, &statusCode, &healthErr, &checkedAt, &downloadSpeedKbps, &manifestTimeMs); err != nil {
		return proxy.Upstream{}, err
	}
	upstream.Enabled = enabled == 1
	upstream.HealthEnabled = healthEnabled == 1
	upstream.HealthStatus = health.HealthStatus(healthStatus)
	upstream.LatencyMs, _ = strconv.ParseInt(latencyMs, 10, 64)
	upstream.StatusCode, _ = strconv.Atoi(statusCode)
	upstream.DownloadSpeedKbps, _ = strconv.ParseFloat(downloadSpeedKbps, 64)
	upstream.ManifestTimeMs, _ = strconv.ParseInt(manifestTimeMs, 10, 64)
	return upstream, nil
}

func (s *Store) ListUpstreams(ctx context.Context) ([]proxy.Upstream, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+upstreamCols+` FROM `+upstreamJoin+` ORDER BY u.registry_prefix, u.priority, u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var upstreams []proxy.Upstream
	for rows.Next() {
		upstream, err := scanUpstreamRow(rows)
		if err != nil {
			return nil, err
		}
		upstreams = append(upstreams, upstream)
	}
	return upstreams, rows.Err()
}

func (s *Store) CreateUpstream(ctx context.Context, input proxy.UpstreamInput) (proxy.Upstream, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return proxy.Upstream{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `INSERT INTO upstreams (registry_prefix, name, base_url, priority, enabled, health_enabled, health_path, speed_test_image, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, input.RegistryPrefix, input.Name, input.BaseURL, input.Priority, boolInt(input.Enabled), boolInt(input.HealthEnabled), input.HealthPath, input.SpeedTestImage, now, now)
	if err != nil {
		return proxy.Upstream{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return proxy.Upstream{}, err
	}
	return s.GetUpstream(ctx, id)
}

func (s *Store) GetUpstream(ctx context.Context, id int64) (proxy.Upstream, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+upstreamCols+` FROM `+upstreamJoin+` WHERE u.id = ?`, id)
	return scanUpstreamRow(row)
}

func (s *Store) UpdateUpstream(ctx context.Context, id int64, input proxy.UpstreamInput) (proxy.Upstream, error) {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return proxy.Upstream{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE upstreams SET registry_prefix = ?, name = ?, base_url = ?, priority = ?, enabled = ?, health_enabled = ?, health_path = ?, speed_test_image = ?, updated_at = ? WHERE id = ?`, input.RegistryPrefix, input.Name, input.BaseURL, input.Priority, boolInt(input.Enabled), boolInt(input.HealthEnabled), input.HealthPath, input.SpeedTestImage, now, id)
	if err != nil {
		return proxy.Upstream{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return proxy.Upstream{}, err
	}
	if rows == 0 {
		return proxy.Upstream{}, sql.ErrNoRows
	}
	return s.GetUpstream(ctx, id)
}

func (s *Store) DeleteUpstream(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM upstreams WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type HealthRecord struct {
	Status     string
	LatencyMs  int64
	StatusCode int
	Error      string
}

func (s *Store) UpdateUpstreamHealth(ctx context.Context, upstreamID int64, record HealthRecord) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO upstream_health (upstream_id, status, latency_ms, status_code, error, checked_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(upstream_id) DO UPDATE SET status=excluded.status, latency_ms=excluded.latency_ms, status_code=excluded.status_code, error=excluded.error, checked_at=excluded.checked_at`,
		upstreamID, record.Status, record.LatencyMs, record.StatusCode, record.Error, now)
	return err
}

func (s *Store) UpdateSpeedTest(ctx context.Context, upstreamID int64, result proxy.SpeedTestResult) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO upstream_health (upstream_id, status, latency_ms, status_code, error, checked_at, download_speed_kbps, manifest_time_ms, speed_test_at)
		VALUES (?, 'unknown', 0, 0, '', ?, ?, ?, ?)
		ON CONFLICT(upstream_id) DO UPDATE SET download_speed_kbps=excluded.download_speed_kbps, manifest_time_ms=excluded.manifest_time_ms, speed_test_at=excluded.speed_test_at`,
		upstreamID, now, result.DownloadSpeedKbps, result.ManifestTimeMs, now)
	return err
}

func (s *Store) GetUpstreamHealth(ctx context.Context, upstreamID int64) (HealthRecord, error) {
	var record HealthRecord
	var checkedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT status, latency_ms, status_code, error, checked_at FROM upstream_health WHERE upstream_id = ?`, upstreamID).Scan(&record.Status, &record.LatencyMs, &record.StatusCode, &record.Error, &checkedAt)
	if err != nil {
		return HealthRecord{}, err
	}
	return record, nil
}

type RequestMetric struct {
	RegistryPrefix string
	UpstreamID     int64
	Method         string
	Path           string
	StatusCode     int
	DurationMs     int64
	Error          string
	Failover       bool
}

func (s *Store) RecordMetric(ctx context.Context, registryPrefix string, upstreamID int64, method, path string, statusCode int, durationMs int64, errMsg string, failover bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO request_metrics (registry_prefix, upstream_id, method, path, status_code, duration_ms, error, failover, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		registryPrefix, upstreamID, method, path, statusCode, durationMs, errMsg, boolInt(failover), now)
	return err
}

type RequestLog struct {
	ID             int64  `json:"id"`
	RegistryPrefix string `json:"registryPrefix"`
	UpstreamID     int64  `json:"upstreamId"`
	UpstreamName   string `json:"upstreamName"`
	Method         string `json:"method"`
	Path           string `json:"path"`
	StatusCode     int    `json:"statusCode"`
	DurationMs     int64  `json:"durationMs"`
	Error          string `json:"error"`
	Failover       bool   `json:"failover"`
	CreatedAt      string `json:"createdAt"`
}

type ListLogsFilter struct {
	RegistryPrefix string
	StatusCode     int // 0 = no filter, 2 = 2xx, 4 = 4xx, 5 = 5xx
	Offset         int
	Limit          int
}

func (s *Store) ListRequestLogs(ctx context.Context, limit int) ([]RequestLog, error) {
	return s.ListRequestLogsWithFilters(ctx, ListLogsFilter{Limit: limit})
}

func (s *Store) ListRequestLogsWithFilters(ctx context.Context, filter ListLogsFilter) ([]RequestLog, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	query := `SELECT r.id, r.registry_prefix, r.upstream_id, COALESCE(u.name, ''), r.method, r.path, r.status_code, r.duration_ms, r.error, r.failover, r.created_at FROM request_metrics r LEFT JOIN upstreams u ON r.upstream_id = u.id`
	var args []any
	var conditions []string

	if filter.RegistryPrefix != "" {
		conditions = append(conditions, "r.registry_prefix = ?")
		args = append(args, filter.RegistryPrefix)
	}
	if filter.StatusCode > 0 {
		conditions = append(conditions, "r.status_code >= ? AND r.status_code < ?")
		args = append(args, filter.StatusCode*100, (filter.StatusCode+1)*100)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY r.id DESC LIMIT ? OFFSET ?"
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []RequestLog
	for rows.Next() {
		var log RequestLog
		var failover int
		if err := rows.Scan(&log.ID, &log.RegistryPrefix, &log.UpstreamID, &log.UpstreamName, &log.Method, &log.Path, &log.StatusCode, &log.DurationMs, &log.Error, &failover, &log.CreatedAt); err != nil {
			return nil, err
		}
		log.Failover = failover == 1
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (s *Store) CountRequestLogs(ctx context.Context, registryPrefix string, statusCode int) (int64, error) {
	query := "SELECT COUNT(*) FROM request_metrics"
	var args []any
	var conditions []string
	if registryPrefix != "" {
		conditions = append(conditions, "registry_prefix = ?")
		args = append(args, registryPrefix)
	}
	if statusCode > 0 {
		conditions = append(conditions, "status_code >= ? AND status_code < ?")
		args = append(args, statusCode*100, (statusCode+1)*100)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	var count int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *Store) DeleteAllRequestLogs(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM request_metrics`)
	return err
}

func (s *Store) DashboardSummary(ctx context.Context) (metrics.DashboardSummary, error) {
	var summary metrics.DashboardSummary
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN u.enabled = 1 AND COALESCE(h.status, 'unknown') != 'unhealthy' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN u.enabled = 1 AND COALESCE(h.status, 'unknown') = 'unhealthy' THEN 1 ELSE 0 END), 0) FROM upstreams u LEFT JOIN upstream_health h ON u.id = h.upstream_id`).Scan(&summary.UpstreamsTotal, &summary.UpstreamsAvailable, &summary.UpstreamsAbnormal); err != nil {
		return metrics.DashboardSummary{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(AVG(duration_ms), 0), COALESCE(SUM(CASE WHEN failover = 1 THEN 1 ELSE 0 END), 0) FROM request_metrics WHERE date(created_at) = date('now')`).Scan(&summary.RequestsToday, &summary.AverageLatencyMs, &summary.FailoversToday); err != nil {
		return metrics.DashboardSummary{}, err
	}
	var failedToday int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN date(created_at) = date('now') AND status_code >= 400 THEN 1 ELSE 0 END), 0) FROM request_metrics`).Scan(&summary.TotalRequests, &failedToday); err != nil {
		return metrics.DashboardSummary{}, err
	}
	if summary.RequestsToday > 0 {
		summary.ErrorRateToday = float64(failedToday) / float64(summary.RequestsToday)
	}
	return summary, nil
}

func scanUpstream(row rowScanner) (proxy.Upstream, error) {
	var upstream proxy.Upstream
	var enabled, healthEnabled int
	if err := row.Scan(&upstream.ID, &upstream.RegistryPrefix, &upstream.Name, &upstream.BaseURL, &upstream.Priority, &enabled, &healthEnabled, &upstream.HealthPath, &upstream.HttpProxy); err != nil {
		return proxy.Upstream{}, err
	}
	upstream.Enabled = enabled == 1
	upstream.HealthEnabled = healthEnabled == 1
	upstream.HealthStatus = health.Unknown
	return upstream, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func IsConstraint(err error) bool {
	return err != nil && (errors.Is(err, sql.ErrNoRows) == false) && fmt.Sprint(err) != ""
}