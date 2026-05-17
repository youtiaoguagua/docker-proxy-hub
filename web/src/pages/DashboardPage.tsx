import { useEffect, useState } from "react";
import { Activity, BarChart3, CheckCircle2, Globe2, XCircle } from "lucide-react";

import { getDashboardSummary } from "@/api/client";
import type { DashboardSummary } from "@/api/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StatCard } from "@/components/StatCard";

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

export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary>(defaultSummary);

  useEffect(() => {
    getDashboardSummary().then(setSummary).catch(() => {});
  }, []);

  return (
    <>
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">仪表盘</h1>
          <p className="mt-2 text-sm text-muted-foreground">Docker 镜像代理系统概览</p>
        </div>
      </div>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="总上游" value={summary.upstreamsTotal} icon={Globe2} tone="bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-400" />
        <StatCard label="可用" value={summary.upstreamsAvailable} icon={CheckCircle2} tone="bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400" />
        <StatCard label="异常" value={summary.upstreamsAbnormal} icon={XCircle} tone="bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400" />
        <StatCard label="今日请求" value={summary.requestsToday} icon={Activity} tone="bg-violet-50 text-violet-600 dark:bg-violet-500/10 dark:text-violet-400" />
      </section>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>使用统计</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
            {([
              ["总请求", summary.totalRequests],
              ["今日请求", summary.requestsToday],
              ["平均延迟", `${summary.averageLatencyMs}ms`],
              ["故障切换", summary.failoversToday],
              ["今日错误率", formatRate(summary.errorRateToday)],
            ] as [string, string | number][]).map(([label, value]) => (
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
            <CardTitle>请求趋势</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex h-72 items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground">
              等待接入真实请求数据
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>延迟趋势</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex h-72 items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground">
              等待接入健康检查与测速数据
            </div>
          </CardContent>
        </Card>
      </section>
    </>
  );
}