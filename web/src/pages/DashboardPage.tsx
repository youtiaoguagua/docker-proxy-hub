import { useEffect, useState } from "react";
import { Activity, BarChart3, CheckCircle2, Gauge, Globe2, XCircle } from "lucide-react";

import { formatError, getMonitoringHealth } from "@/api/client";
import type { DashboardSummary, Upstream } from "@/api/types";
import { StatCard } from "@/components/StatCard";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const defaultSummary: DashboardSummary = {
  upstreamsTotal: 0,
  upstreamsAvailable: 0,
  upstreamsAbnormal: 0,
  requestsToday: 0,
  totalRequests: 0,
  averageLatencyMs: 0,
  failoversToday: 0,
  errorRateToday: 0,
};

function formatRate(rate: number): string {
  return (rate * 100).toFixed(1) + "%";
}

function healthBadge(status: Upstream["healthStatus"]) {
  switch (status) {
    case "healthy":
      return <Badge variant="success">健康</Badge>;
    case "unhealthy":
      return <Badge variant="warning">异常</Badge>;
    default:
      return <Badge>未知</Badge>;
  }
}

function healthDetail(upstream: Upstream) {
  if (!upstream.enabled) return "上游已禁用";
  if (!upstream.healthEnabled) return "健康检查已关闭";
  if (upstream.statusCode > 0) return `HTTP ${upstream.statusCode}`;
  if (upstream.healthStatus === "unknown") return "未检查";
  return "无响应状态码";
}

function formatLatency(upstream: Upstream) {
  if (upstream.healthStatus === "unknown") return "-";
  if (upstream.latencyMs == null) return "-";
  if (upstream.latencyMs < 1000) return `${upstream.latencyMs}ms`;
  return `${(upstream.latencyMs / 1000).toFixed(1)}s`;
}

function formatSummaryLatency(latencyMs: number) {
  if (latencyMs < 1000) return `${latencyMs.toFixed(0)}ms`;
  return `${(latencyMs / 1000).toFixed(1)}s`;
}

function formatSpeed(kbps: number | null | undefined) {
  if (!kbps) return "-";
  if (kbps < 1024) return `${kbps.toFixed(0)} KB/s`;
  return `${(kbps / 1024).toFixed(1)} MB/s`;
}

function PanelMessage({ message }: { message: string }) {
  return (
    <div className="flex h-72 items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground">
      {message}
    </div>
  );
}

export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary>(defaultSummary);
  const [upstreams, setUpstreams] = useState<Upstream[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;

    getMonitoringHealth()
      .then((data) => {
        if (cancelled) return;
        setSummary(data.summary);
        setUpstreams(data.upstreams ?? []);
        setError("");
        setLoaded(true);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(formatError(err));
        setLoaded(false);
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const visibleUpstreams = upstreams.slice(0, 6);
  const summaryItems: [string, string | number][] = [
    ["总请求", loaded ? summary.totalRequests : "-"],
    ["今日请求", loaded ? summary.requestsToday : "-"],
    ["平均延迟", loaded ? formatSummaryLatency(summary.averageLatencyMs) : "-"],
    ["故障切换", loaded ? summary.failoversToday : "-"],
    ["今日错误率", loaded ? formatRate(summary.errorRateToday) : "-"],
  ];

  return (
    <>
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">仪表盘</h1>
          <p className="mt-2 text-sm text-muted-foreground">Docker 镜像代理系统概览</p>
        </div>
      </div>

      {error && <p className="mb-4 text-sm text-destructive">{error}</p>}
      {loading && <p className="mb-4 text-sm text-muted-foreground">加载监控数据中...</p>}

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="总上游" value={loaded ? summary.upstreamsTotal : "-"} icon={Globe2} tone="bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-400" />
        <StatCard label="可用" value={loaded ? summary.upstreamsAvailable : "-"} icon={CheckCircle2} tone="bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400" />
        <StatCard label="异常" value={loaded ? summary.upstreamsAbnormal : "-"} icon={XCircle} tone="bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400" />
        <StatCard label="今日请求" value={loaded ? summary.requestsToday : "-"} icon={Activity} tone="bg-violet-50 text-violet-600 dark:bg-violet-500/10 dark:text-violet-400" />
      </section>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>使用统计</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
            {summaryItems.map(([label, value]) => (
              <div key={label} className="rounded-xl bg-muted p-5">
                <div className="flex items-center gap-4">
                  <div className="rounded-xl bg-blue-50 p-3 text-blue-600 dark:bg-blue-500/10 dark:text-blue-400">
                    <BarChart3 className="h-5 w-5" />
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">{label}</p>
                    <p className="text-xl font-semibold">{value}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <section className="mt-6 grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>上游健康概览</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <PanelMessage message="加载健康状态中..." />
            ) : !loaded ? (
              <PanelMessage message="监控数据加载失败" />
            ) : visibleUpstreams.length === 0 ? (
              <PanelMessage message="暂无上游配置" />
            ) : (
              <div className="space-y-3">
                {visibleUpstreams.map((upstream) => (
                  <div key={upstream.id} className="flex items-start justify-between gap-3 rounded-xl border p-4">
                    <div className="min-w-0">
                      <p className="truncate font-medium">{upstream.name}</p>
                      <p className="mt-1 text-xs text-muted-foreground">{healthDetail(upstream)}</p>
                    </div>
                    {healthBadge(upstream.healthStatus)}
                  </div>
                ))}
                {upstreams.length > visibleUpstreams.length && (
                  <p className="text-xs text-muted-foreground">还有 {upstreams.length - visibleUpstreams.length} 个上游未展示</p>
                )}
              </div>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>延迟与测速概览</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <PanelMessage message="加载延迟与测速数据中..." />
            ) : !loaded ? (
              <PanelMessage message="监控数据加载失败" />
            ) : visibleUpstreams.length === 0 ? (
              <PanelMessage message="暂无上游配置" />
            ) : (
              <div className="space-y-3">
                {visibleUpstreams.map((upstream) => (
                  <div key={upstream.id} className="flex items-center justify-between gap-4 rounded-xl border p-4">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <Gauge className="h-4 w-4 text-muted-foreground" />
                        <p className="truncate font-medium">{upstream.name}</p>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">{upstream.registryPrefix}</p>
                    </div>
                    <div className="space-y-1 text-right text-sm">
                      <p>
                        <span className="text-muted-foreground">延迟 </span>
                        <span className="font-medium">{formatLatency(upstream)}</span>
                      </p>
                      <p>
                        <span className="text-muted-foreground">速度 </span>
                        <span className="font-medium">{formatSpeed(upstream.downloadSpeedKbps)}</span>
                      </p>
                    </div>
                  </div>
                ))}
                {upstreams.length > visibleUpstreams.length && (
                  <p className="text-xs text-muted-foreground">还有 {upstreams.length - visibleUpstreams.length} 个上游未展示</p>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </section>
    </>
  );
}
