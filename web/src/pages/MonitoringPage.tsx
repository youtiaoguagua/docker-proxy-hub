import { useCallback, useEffect, useRef, useState } from "react";
import { Activity, CheckCircle2, Clock, RefreshCw, XCircle } from "lucide-react";

import { formatError, getMonitoringHealth, getRequestLogs } from "@/api/client";
import type { RequestLog, Upstream } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function MonitoringPage() {
  const [upstreams, setUpstreams] = useState<Upstream[]>([]);
  const [logs, setLogs] = useState<RequestLog[]>([]);
  const [error, setError] = useState("");
  const [loadingLogs, setLoadingLogs] = useState(false);
  const loadedRef = useRef(false);

  const loadLogs = useCallback(() => {
    setLoadingLogs(true);
    getRequestLogs(100)
      .then((data) => { setLogs(data.logs ?? []); })
      .catch(() => {})
      .finally(() => { setLoadingLogs(false); });
  }, []);

  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;
    getMonitoringHealth()
      .then((data) => { setUpstreams(data.upstreams ?? []); })
      .catch(() => { setError(formatError(new Error("加载监控数据失败"))); });
    loadLogs();
  }, [loadLogs]);

  const healthyCount = upstreams.filter((u) => u.healthStatus === "healthy").length;
  const unhealthyCount = upstreams.filter((u) => u.healthStatus === "unhealthy").length;
  const unknownCount = upstreams.filter((u) => u.healthStatus === "unknown" || !u.healthStatus).length;

  const healthBadge = (status: string) => {
    switch (status) {
      case "healthy":
        return <Badge variant="success">健康</Badge>;
      case "unhealthy":
        return <Badge variant="warning">异常</Badge>;
      default:
        return <Badge>未知</Badge>;
    }
  };

  const statusBadge = (code: number) => {
    if (code < 300) return <Badge variant="success">{code}</Badge>;
    if (code < 400) return <Badge>{code}</Badge>;
    return <Badge variant="danger">{code}</Badge>;
  };

  const formatTime = (iso: string) => {
    if (!iso) return "-";
    const d = new Date(iso);
    return d.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" });
  };

  return (
    <>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">监控日志</h1>
        <p className="mt-2 text-sm text-muted-foreground">健康检查、请求日志与故障切换记录</p>
      </div>

      {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

      <section className="mb-6 grid gap-4 md:grid-cols-3">
        <Card>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="rounded-xl bg-emerald-50 p-3 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400">
              <CheckCircle2 className="h-6 w-6" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">健康</p>
              <p className="text-2xl font-semibold">{healthyCount}</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="rounded-xl bg-red-50 p-3 text-red-600 dark:bg-red-500/10 dark:text-red-400">
              <XCircle className="h-6 w-6" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">异常</p>
              <p className="text-2xl font-semibold">{unhealthyCount}</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="rounded-xl bg-muted p-3 text-muted-foreground">
              <Clock className="h-6 w-6" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">未知</p>
              <p className="text-2xl font-semibold">{unknownCount}</p>
            </div>
          </CardContent>
        </Card>
      </section>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            上游健康状态
          </CardTitle>
        </CardHeader>
        <CardContent>
          {upstreams.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Activity className="mb-2 h-10 w-10" />
              <p>暂无上游配置</p>
              <p className="text-sm">请先添加上游后再查看健康状态</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>仓库类型</TableHead>
                  <TableHead>地址</TableHead>
                  <TableHead>健康状态</TableHead>
                  <TableHead>启用</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {upstreams.map((upstream) => (
                  <TableRow key={upstream.id}>
                    <TableCell className="font-medium">{upstream.name}</TableCell>
                    <TableCell><Badge>{upstream.registryPrefix}</Badge></TableCell>
                    <TableCell className="max-w-48 truncate">{upstream.baseUrl}</TableCell>
                    <TableCell>{healthBadge(upstream.healthStatus)}</TableCell>
                    <TableCell>
                      <Badge variant={upstream.enabled ? "success" : "warning"}>
                        {upstream.enabled ? "启用" : "禁用"}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>请求日志</CardTitle>
          <Button variant="outline" size="sm" onClick={loadLogs} disabled={loadingLogs}>
            <RefreshCw className={`mr-1 h-4 w-4 ${loadingLogs ? "animate-spin" : ""}`} />
            刷新
          </Button>
        </CardHeader>
        <CardContent>
          {logs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Activity className="mb-2 h-10 w-10" />
              <p>暂无请求记录</p>
              <p className="text-sm">代理请求将在此处显示</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>时间</TableHead>
                  <TableHead>方法</TableHead>
                  <TableHead>路径</TableHead>
                  <TableHead>仓库</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>延迟</TableHead>
                  <TableHead>故障切换</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.slice(0, 50).map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="text-xs whitespace-nowrap">{formatTime(log.createdAt)}</TableCell>
                    <TableCell className="font-mono text-xs">{log.method}</TableCell>
                    <TableCell className="max-w-40 truncate text-xs">{log.path}</TableCell>
                    <TableCell><Badge>{log.registryPrefix}</Badge></TableCell>
                    <TableCell>{statusBadge(log.statusCode)}</TableCell>
                    <TableCell className="text-xs">{log.durationMs}ms</TableCell>
                    <TableCell>{log.failover ? <Badge variant="warning">是</Badge> : <span className="text-muted-foreground">-</span>}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </>
  );
}