package metrics

type RequestSummary struct {
	Total  int64
	Failed int64
}

type DashboardSummary struct {
	UpstreamsTotal     int64   `json:"upstreamsTotal"`
	UpstreamsAvailable int64   `json:"upstreamsAvailable"`
	UpstreamsAbnormal  int64   `json:"upstreamsAbnormal"`
	RequestsToday      int64   `json:"requestsToday"`
	TotalRequests      int64   `json:"totalRequests"`
	AverageLatencyMs   int64   `json:"averageLatencyMs"`
	FailoversToday     int64   `json:"failoversToday"`
	ErrorRateToday     float64 `json:"errorRateToday"`
}
