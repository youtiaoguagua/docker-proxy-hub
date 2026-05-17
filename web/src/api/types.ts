export type Admin = {
  id: number;
  username: string;
};

export type DashboardSummary = {
  upstreamsTotal: number;
  upstreamsAvailable: number;
  upstreamsAbnormal: number;
  requestsToday: number;
  totalRequests: number;
  averageLatencyMs: number;
  failoversToday: number;
  errorRateToday: number;
};

export type Upstream = {
  id: number;
  registryPrefix: string;
  name: string;
  baseUrl: string;
  priority: number;
  enabled: boolean;
  healthEnabled: boolean;
  healthPath: string;
  speedTestImage: string;
  healthStatus: string;
  latencyMs: number;
  statusCode: number;
  downloadSpeedKbps: number;
  manifestTimeMs: number;
};

export type UpstreamInput = {
  registryPrefix: string;
  name: string;
  baseUrl: string;
  priority: number;
  enabled: boolean;
  healthEnabled: boolean;
  healthPath: string;
  speedTestImage: string;
};

export type SetupStatus = {
  setupRequired: boolean;
};

export type MonitoringData = {
  upstreams: Upstream[];
  health: Upstream[];
  summary: DashboardSummary;
};

export type RequestLog = {
  id: number;
  registryPrefix: string;
  upstreamId: number;
  upstreamName: string;
  method: string;
  path: string;
  statusCode: number;
  durationMs: number;
  error: string;
  failover: boolean;
  createdAt: string;
};

export type ApiError = {
  error: {
    code: string;
    message: string;
  };
};