package health

type HealthStatus string

const (
	Unknown   HealthStatus = "unknown"
	Healthy   HealthStatus = "healthy"
	Unhealthy HealthStatus = "unhealthy"
)
