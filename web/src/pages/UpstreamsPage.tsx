import { useCallback, useEffect, useState } from "react";
import { Activity, Gauge, Globe2, Plus, RefreshCw, Trash2 } from "lucide-react";

import { checkUpstreamHealth, checkUpstreamHealthOne, createUpstream, deleteUpstream, formatError, listUpstreams, speedTestUpstream, updateUpstream } from "@/api/client";
import type { Upstream, UpstreamInput } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const registryPrefixes = ["docker.io", "ghcr.io", "gcr.io", "quay.io", "registry.k8s.io"];

const emptyInput: UpstreamInput = {
  registryPrefix: "docker.io",
  name: "",
  baseUrl: "",
  priority: 100,
  enabled: true,
  healthEnabled: true,
  healthPath: "/v2/",
  httpProxy: "",
  speedTestImage: "library/alpine",
};

export function UpstreamsPage() {
  const [upstreams, setUpstreams] = useState<Upstream[]>([]);
  const [error, setError] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<UpstreamInput>({ ...emptyInput });
  const [formError, setFormError] = useState("");
  const [loading, setLoading] = useState(false);
  const [checkingHealth, setCheckingHealth] = useState(false);
  const [checkingOne, setCheckingOne] = useState<number | null>(null);
  const [speedTesting, setSpeedTesting] = useState<number | null>(null);

  useEffect(() => {
    listUpstreams()
      .then((result) => { setUpstreams(result.upstreams ?? []); })
      .catch(() => { setError("加载上游列表失败"); });
  }, []);

  const refresh = useCallback(() => {
    listUpstreams()
      .then((result) => { setUpstreams(result.upstreams ?? []); })
      .catch(() => { setError("加载上游列表失败"); });
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError("");
    setLoading(true);
    try {
      if (editId !== null) {
        await updateUpstream(editId, form);
      } else {
        await createUpstream(form);
      }
      setDialogOpen(false);
      setEditId(null);
      setForm({ ...emptyInput });
      refresh();
    } catch (err) {
      setFormError(formatError(err));
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("确定删除此上游？")) return;
    try {
      await deleteUpstream(id);
      refresh();
    } catch {
      setError("删除失败");
    }
  };

  const startEdit = (upstream: Upstream) => {
    setEditId(upstream.id);
    setForm({
      registryPrefix: upstream.registryPrefix,
      name: upstream.name,
      baseUrl: upstream.baseUrl,
      priority: upstream.priority,
      enabled: upstream.enabled,
      healthEnabled: upstream.healthEnabled,
      healthPath: upstream.healthPath,
      httpProxy: upstream.httpProxy,
      speedTestImage: upstream.speedTestImage || "",
    });
    setDialogOpen(true);
  };

  const startCreate = () => {
    setEditId(null);
    setForm({ ...emptyInput });
    setFormError("");
    setDialogOpen(true);
  };

  const handleCheckHealth = async () => {
    setCheckingHealth(true);
    try {
      const result = await checkUpstreamHealth();
      setUpstreams(result.upstreams ?? []);
    } catch {
      setError("健康检查失败");
    } finally {
      setCheckingHealth(false);
    }
  };

  const handleCheckOne = async (id: number) => {
    setCheckingOne(id);
    try {
      const result = await checkUpstreamHealthOne(id);
      setUpstreams((prev) => prev.map((u) => u.id === id ? result.upstream : u));
    } catch {
      // single check failed, ignore
    } finally {
      setCheckingOne(null);
    }
  };

  const handleSpeedTest = async (id: number) => {
    setSpeedTesting(id);
    try {
      const result = await speedTestUpstream(id);
      setUpstreams((prev) => prev.map((u) => u.id === id ? result.upstream : u));
    } catch (err) {
      setError(formatError(err));
    } finally {
      setSpeedTesting(null);
    }
  };

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

  const formatLatency = (ms: number | null | undefined) => {
    if (ms == null) return "-";
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  };

  const formatSpeed = (kbps: number | null | undefined) => {
    if (!kbps) return "-";
    if (kbps < 1024) return `${kbps.toFixed(0)} KB/s`;
    return `${(kbps / 1024).toFixed(1)} MB/s`;
  };

  return (
    <>
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">上游管理</h1>
          <p className="mt-2 text-sm text-muted-foreground">管理镜像源上游加速站</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleCheckHealth} disabled={checkingHealth}>
            {checkingHealth ? <RefreshCw className="mr-1 h-4 w-4 animate-spin" /> : <Activity className="mr-1 h-4 w-4" />}
            {checkingHealth ? "检查中..." : "健康检查"}
          </Button>
          <Button onClick={startCreate}>
            <Plus className="mr-1 h-4 w-4" />
            添加上游
          </Button>
        </div>
      </div>

      {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

      <Dialog open={dialogOpen} onOpenChange={(open) => { setDialogOpen(open); if (!open) { setEditId(null); setForm({ ...emptyInput }); setFormError(""); } }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editId !== null ? "编辑上游" : "添加上游"}</DialogTitle>
          </DialogHeader>
          {formError && <p className="text-sm text-destructive">{formError}</p>}
          <form onSubmit={handleSubmit} className="grid gap-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>仓库类型</Label>
                <select
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  value={form.registryPrefix}
                  onChange={(e) => setForm({ ...form, registryPrefix: e.target.value })}
                >
                  {registryPrefixes.map((prefix) => (
                    <option key={prefix} value={prefix}>{prefix}</option>
                  ))}
                </select>
              </div>
              <div className="space-y-2">
                <Label>名称</Label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="阿里云镜像站" required />
              </div>
            </div>
            <div className="space-y-2">
              <Label>上游地址</Label>
              <Input value={form.baseUrl} onChange={(e) => setForm({ ...form, baseUrl: e.target.value })} placeholder="mirror.example.com（自动补 https://）" required />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>优先级（越小越优先）</Label>
                <Input type="number" value={form.priority} onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} required />
              </div>
              <div className="space-y-2">
                <Label>健康检查路径</Label>
                <Input value={form.healthPath} onChange={(e) => setForm({ ...form, healthPath: e.target.value })} required />
              </div>
            </div>
            <div className="space-y-2">
              <Label>测速镜像（可选，如 library/alpine）</Label>
              <Input value={form.speedTestImage} onChange={(e) => setForm({ ...form, speedTestImage: e.target.value })} placeholder="library/alpine" />
            </div>
            <div className="space-y-2">
              <Label>HTTP 代理（可选）</Label>
              <Input value={form.httpProxy} onChange={(e) => setForm({ ...form, httpProxy: e.target.value })} placeholder="http://proxy:8080" />
            </div>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} />
                启用
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={form.healthEnabled} onChange={(e) => setForm({ ...form, healthEnabled: e.target.checked })} />
                健康检查
              </label>
            </div>
            <div className="flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
              <Button type="submit" disabled={loading}>{loading ? "提交中..." : "保存"}</Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      <Card>
        <CardHeader>
          <CardTitle>上游列表</CardTitle>
        </CardHeader>
        <CardContent>
          {upstreams.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Globe2 className="mb-2 h-10 w-10" />
              <p>暂无上游配置</p>
              <p className="text-sm">点击上方"添加上游"按钮创建第一个上游</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>仓库类型</TableHead>
                  <TableHead>地址</TableHead>
                  <TableHead>优先级</TableHead>
                  <TableHead>健康</TableHead>
                  <TableHead>延迟</TableHead>
                  <TableHead>测速</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {upstreams.map((upstream) => (
                  <TableRow key={upstream.id}>
                    <TableCell className="font-medium">{upstream.name}</TableCell>
                    <TableCell><Badge>{upstream.registryPrefix}</Badge></TableCell>
                    <TableCell className="max-w-48 truncate">{upstream.baseUrl}</TableCell>
                    <TableCell>{upstream.priority}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        {healthBadge(upstream.healthStatus)}
                        <Button variant="outline" size="sm" className="h-6 px-1.5 text-[11px]" onClick={() => handleCheckOne(upstream.id)} disabled={checkingOne === upstream.id} title={checkingOne === upstream.id ? "检查中..." : "检查健康"}>
                          <RefreshCw className={`mr-0.5 h-2.5 w-2.5 ${checkingOne === upstream.id ? "animate-spin" : ""}`} />
                          检查
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell>{formatLatency(upstream.latencyMs)}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <span className="text-sm">{formatSpeed(upstream.downloadSpeedKbps)}</span>
                        <Button variant="outline" size="sm" className="h-6 px-1.5 text-[11px]" onClick={() => handleSpeedTest(upstream.id)} disabled={speedTesting === upstream.id} title={speedTesting === upstream.id ? "测速中..." : "测速"}>
                          {speedTesting === upstream.id
                            ? <RefreshCw className="mr-0.5 h-2.5 w-2.5 animate-spin" />
                            : <Gauge className="mr-0.5 h-2.5 w-2.5" />}
                          测速
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={upstream.enabled ? "success" : "warning"}>
                        {upstream.enabled ? "启用" : "禁用"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button variant="outline" size="sm" onClick={() => startEdit(upstream)}>编辑</Button>
                        <Button variant="outline" size="sm" onClick={() => handleDelete(upstream.id)}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
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