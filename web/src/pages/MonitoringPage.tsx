import { useCallback, useEffect, useRef, useState } from "react";
import { Activity, RefreshCw, Trash2 } from "lucide-react";

import { deleteRequestLogs, getRequestLogs } from "@/api/client";
import type { RequestLog } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const PAGE_SIZE = 50;

const registryPrefixes = ["docker.io", "ghcr.io", "gcr.io", "quay.io", "registry.k8s.io"];

const statusOptions = [
  { value: 0, label: "全部" },
  { value: 2, label: "2xx 成功" },
  { value: 4, label: "4xx 客户端错误" },
  { value: 5, label: "5xx 服务端错误" },
];

export function MonitoringPage() {
  const [logs, setLogs] = useState<RequestLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [registry, setRegistry] = useState("");
  const [status, setStatus] = useState(0);
  const [error, setError] = useState("");
  const [loadingLogs, setLoadingLogs] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const loadedRef = useRef(false);

  const loadLogs = useCallback((p: number, reg: string, st: number) => {
    setLoadingLogs(true);
    getRequestLogs({ limit: PAGE_SIZE, offset: p * PAGE_SIZE, registry: reg, status: st })
      .then((data) => { setLogs(data.logs ?? []); setTotal(data.total ?? 0); })
      .catch(() => { setError("加载日志失败"); })
      .finally(() => { setLoadingLogs(false); });
  }, []);

  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;
    loadLogs(0, "", 0);
  }, [loadLogs]);

  const handleFilter = () => {
    setPage(0);
    loadLogs(0, registry, status);
  };

  const handleReset = () => {
    setRegistry("");
    setStatus(0);
    setPage(0);
    loadLogs(0, "", 0);
  };

  const handleClear = async () => {
    setClearing(true);
    try {
      await deleteRequestLogs();
      setLogs([]);
      setTotal(0);
      setPage(0);
      setConfirmOpen(false);
    } catch {
      setError("清空日志失败");
    } finally {
      setClearing(false);
    }
  };

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const start = total === 0 ? 0 : page * PAGE_SIZE + 1;
  const end = Math.min((page + 1) * PAGE_SIZE, total);

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
        <p className="mt-2 text-sm text-muted-foreground">请求日志与故障切换记录</p>
      </div>

      {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

      <Card className="mb-6">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">筛选条件</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-end gap-4">
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">仓库类型</label>
              <select
                className="flex h-9 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={registry}
                onChange={(e) => setRegistry(e.target.value)}
              >
                <option value="">全部</option>
                {registryPrefixes.map((p) => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
            </div>
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">状态码</label>
              <select
                className="flex h-9 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={status}
                onChange={(e) => setStatus(Number(e.target.value))}
              >
                {statusOptions.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={handleFilter}>筛选</Button>
              <Button variant="ghost" size="sm" onClick={handleReset}>重置</Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>请求日志</CardTitle>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => loadLogs(page, registry, status)} disabled={loadingLogs}>
              <RefreshCw className={`mr-1 h-4 w-4 ${loadingLogs ? "animate-spin" : ""}`} />
              刷新
            </Button>
            <Button variant="destructive" size="sm" onClick={() => setConfirmOpen(true)} disabled={clearing || total === 0}>
              <Trash2 className="mr-1 h-4 w-4" />
              清空
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {logs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Activity className="mb-2 h-10 w-10" />
              <p>暂无请求记录</p>
              <p className="text-sm">代理请求将在此处显示</p>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>时间</TableHead>
                    <TableHead>方法</TableHead>
                    <TableHead>路径</TableHead>
                    <TableHead>仓库</TableHead>
                    <TableHead>上游</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>延迟</TableHead>
                    <TableHead>故障切换</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log) => (
                    <TableRow key={log.id}>
                      <TableCell className="text-xs whitespace-nowrap">{formatTime(log.createdAt)}</TableCell>
                      <TableCell className="font-mono text-xs">{log.method}</TableCell>
                      <TableCell className="max-w-40 truncate text-xs">{log.path}</TableCell>
                      <TableCell><Badge>{log.registryPrefix}</Badge></TableCell>
                      <TableCell className="text-xs">{log.upstreamName || (log.upstreamId ? `#${log.upstreamId}` : "-")}</TableCell>
                      <TableCell>{statusBadge(log.statusCode)}</TableCell>
                      <TableCell className="text-xs">{log.durationMs}ms</TableCell>
                      <TableCell>{log.failover ? <Badge variant="warning">是</Badge> : <span className="text-muted-foreground">-</span>}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <div className="flex items-center justify-between pt-4">
                <p className="text-sm text-muted-foreground">
                  第 {start}-{end} 条，共 {total} 条
                </p>
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={() => { const p = page - 1; setPage(p); loadLogs(p, registry, status); }} disabled={page <= 0}>
                    上一页
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => { const p = page + 1; setPage(p); loadLogs(p, registry, status); }} disabled={page >= totalPages - 1}>
                    下一页
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认清空日志</DialogTitle>
            <DialogDescription>此操作将删除所有请求日志且不可恢复，确定继续？</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)} disabled={clearing}>取消</Button>
            <Button variant="destructive" onClick={handleClear} disabled={clearing}>{clearing ? "清空中..." : "确认清空"}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}