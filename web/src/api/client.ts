import type { Admin, DashboardSummary, MonitoringData, RequestLog, SetupStatus, Upstream, UpstreamInput } from "./types";

const API_BASE = "";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: { code: "unknown", message: response.statusText } }));
    throw body;
  }
  return response.json();
}

export function getSetupStatus(): Promise<SetupStatus> {
  return request<SetupStatus>("/api/setup/status");
}

export function setupAdmin(username: string, password: string): Promise<{ admin: Admin }> {
  return request<{ admin: Admin }>("/api/setup", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export function login(username: string, password: string): Promise<{ admin: Admin }> {
  return request<{ admin: Admin }>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export function logout(): Promise<void> {
  return request<void>("/api/auth/logout", { method: "POST" });
}

export function getMe(): Promise<{ admin: Admin }> {
  return request<{ admin: Admin }>("/api/auth/me");
}

export async function changeAdmin(currentPassword: string, username?: string, password?: string): Promise<{ admin: Admin }> {
  const result = await request<{ admin: Admin }>("/api/auth/admin", {
    method: "PUT",
    body: JSON.stringify({ currentPassword, username: username ?? "", password: password ?? "" }),
  });
  return result;
}

export function getDashboardSummary(): Promise<DashboardSummary> {
  return request<DashboardSummary>("/api/dashboard/summary");
}

export function listUpstreams(): Promise<{ upstreams: Upstream[] }> {
  return request<{ upstreams: Upstream[] }>("/api/upstreams");
}

export function createUpstream(input: UpstreamInput): Promise<{ upstream: Upstream }> {
  return request<{ upstream: Upstream }>("/api/upstreams", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateUpstream(id: number, input: UpstreamInput): Promise<{ upstream: Upstream }> {
  return request<{ upstream: Upstream }>(`/api/upstreams/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteUpstream(id: number): Promise<void> {
  return request<void>(`/api/upstreams/${id}`, { method: "DELETE" });
}

export function checkUpstreamHealth(): Promise<{ upstreams: Upstream[] }> {
  return request<{ upstreams: Upstream[] }>("/api/upstreams/check-health", { method: "POST" });
}

export function checkUpstreamHealthOne(id: number): Promise<{ upstream: Upstream }> {
  return request<{ upstream: Upstream }>(`/api/upstreams/${id}/check-health`, { method: "POST" });
}

export function speedTestUpstream(id: number): Promise<{ upstream: Upstream }> {
  return request<{ upstream: Upstream }>(`/api/upstreams/${id}/speed-test`, { method: "POST" });
}

export function getMonitoringHealth(): Promise<MonitoringData> {
  return request<MonitoringData>("/api/monitoring/health");
}

export function getRequestLogs(limit = 50): Promise<{ logs: RequestLog[] }> {
  return request<{ logs: RequestLog[] }>(`/api/monitoring/logs?limit=${limit}`);
}

export function formatError(error: unknown): string {
  if (error && typeof error === "object" && "error" in error) {
    const apiError = error as { error: { code: string; message: string } };
    return apiError.error.message;
  }
  return "请求失败，请稍后重试";
}