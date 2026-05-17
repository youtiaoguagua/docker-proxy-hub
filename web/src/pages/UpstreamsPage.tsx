import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Activity, CheckCircle2, ChevronDown, ChevronRight, Gauge, Globe2, Plus, RefreshCw, Trash2, XCircle } from "lucide-react";

import { checkUpstreamHealth, checkUpstreamHealthOne, createUpstream, deleteUpstream, formatError, listUpstreams, speedTestUpstream, updateUpstream } from "@/api/client";
import type { Upstream, UpstreamInput } from "@/api/types";
import { StatCard } from "@/components/StatCard";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const registryPrefixes = ["docker.io", "ghcr.io", "gcr.io", "quay.io", "registry.k8s.io"];

const defaultSpeedTestImages: Record<string, string> = {
  "docker.io": "library/alpine",
  "ghcr.io": "stefanprodan/podinfo",
  "gcr.io": "google-containers/pause",
  "quay.io": "prometheus/prometheus",
  "registry.k8s.io": "pause",
};

const getDefaultSpeedTestImage = (registryPrefix: string) => defaultSpeedTestImages[registryPrefix] ?? "";

const emptyInput: UpstreamInput = {
  registryPrefix: "docker.io",
  name: "",
  baseUrl: "",
  priority: 100,
  enabled: true,
  healthEnabled: true,
  healthPath: "/v2/",
  speedTestImage: getDefaultSpeedTestImage("docker.io"),
};

export function UpstreamsPage() {
  const [upstreams, setUpstreams] = useState<Upstream[]>([]);
  const [error, setError] = useState("");
  const [initialLoading, setInitialLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<UpstreamInput>({ ...emptyInput });
  const [formError, setFormError] = useState("");
  const [loading, setLoading] = useState(false);
  const [checkingHealth, setCheckingHealth] = useState(false);
  const [checkingOne, setCheckingOne] = useState<number | null>(null);
  const [speedTesting, setSpeedTesting] = useState<number | null>(null);
  const [savingPriorityId, setSavingPriorityId] = useState<number | null>(null);
  const [togglingEnabledId, setTogglingEnabledId] = useState<number | null>(null);
  const [priorityDrafts, setPriorityDrafts] = useState<Record<number, string>>({});
  const [deleteTarget, setDeleteTarget] = useState<Upstream | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [openSections, setOpenSections] = useState<Record<string, boolean>>({});

  const loadUpstreams = useCallback(async (options?: { initial?: boolean }) => {
    if (options?.initial) {
      setInitialLoading(true);
    }
    try {
      const result = await listUpstreams();
      setUpstreams(result.upstreams ?? []);
      setError("");
    } catch (err) {
      setError(formatError(err));
    } finally {
      if (options?.initial) {
        setInitialLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    void loadUpstreams({ initial: true });
  }, [loadUpstreams]);

  const refresh = useCallback(async () => {
    await loadUpstreams();
  }, [loadUpstreams]);

  const summary = useMemo(() => {
    return {
      total: upstreams.length,
      enabled: upstreams.filter((upstream) => upstream.enabled).length,
      healthy: upstreams.filter((upstream) => upstream.healthStatus === "healthy").length,
      attention: upstreams.filter((upstream) => upstream.healthStatus !== "healthy").length,
    };
  }, [upstreams]);

  const groupedUpstreams = useMemo(() => {
    const extras = Array.from(new Set(upstreams.map((upstream) => upstream.registryPrefix).filter((prefix) => !registryPrefixes.includes(prefix)))).sort();
    const orderedPrefixes = [...registryPrefixes, ...extras];
    return orderedPrefixes
      .map((prefix) => ({ prefix, items: upstreams.filter((upstream) => upstream.registryPrefix === prefix) }))
      .filter((group) => group.items.length > 0);
  }, [upstreams]);

  const isSectionOpen = (prefix: string) => openSections[prefix] ?? true;

  const toggleSection = (prefix: string) => {
    setOpenSections((current) => ({ ...current, [prefix]: !(current[prefix] ?? true) }));
  };

  const applyUpstreamUpdate = (nextUpstream: Upstream) => {
    setUpstreams((prev) => prev.map((upstream) => upstream.id === nextUpstream.id ? nextUpstream : upstream));
  };

  const toUpstreamInput = (upstream: Upstream, overrides: Partial<UpstreamInput> = {}): UpstreamInput => ({
    registryPrefix: upstream.registryPrefix,
    name: upstream.name,
    baseUrl: upstream.baseUrl,
    priority: upstream.priority,
    enabled: upstream.enabled,
    healthEnabled: upstream.healthEnabled,
    healthPath: upstream.healthPath,
    speedTestImage: upstream.speedTestImage,
    ...overrides,
  });

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
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
      await refresh();
    } catch (err) {
      setFormError(formatError(err));
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteRequest = (upstream: Upstream) => {
    setDeleteTarget(upstream);
  };

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) {
      return;
    }
    setDeletingId(deleteTarget.id);
    try {
      await deleteUpstream(deleteTarget.id);
      setDeleteTarget(null);
      await refresh();
    } catch (err) {
      setError(formatError(err));
    } finally {
      setDeletingId(null);
    }
  };

  const startEdit = (upstream: Upstream) => {
    setEditId(upstream.id);
    setFormError("");
    setForm({
      registryPrefix: upstream.registryPrefix,
      name: upstream.name,
      baseUrl: upstream.baseUrl,
      priority: upstream.priority,
      enabled: upstream.enabled,
      healthEnabled: upstream.healthEnabled,
      healthPath: upstream.healthPath,
      speedTestImage: upstream.speedTestImage || getDefaultSpeedTestImage(upstream.registryPrefix),
    });
    setDialogOpen(true);
  };

  const startCreate = () => {
    setEditId(null);
    setForm({ ...emptyInput });
    setFormError("");
    setDialogOpen(true);
  };

  const handleRegistryPrefixChange = (registryPrefix: string) => {
    setForm((current) => {
      const previousDefault = getDefaultSpeedTestImage(current.registryPrefix);
      const nextDefault = getDefaultSpeedTestImage(registryPrefix);
      const shouldUpdateSpeedTestImage = current.speedTestImage === "" || current.speedTestImage === previousDefault;
      return {
        ...current,
        registryPrefix,
        speedTestImage: shouldUpdateSpeedTestImage ? nextDefault : current.speedTestImage,
      };
    });
  };

  const handleCheckHealth = async () => {
    setCheckingHealth(true);
    try {
      const result = await checkUpstreamHealth();
      setUpstreams(result.upstreams ?? []);
      setError("");
    } catch (err) {
      setError(formatError(err));
    } finally {
      setCheckingHealth(false);
    }
  };

  const handleCheckOne = async (id: number) => {
    setCheckingOne(id);
    try {
      const result = await checkUpstreamHealthOne(id);
      setUpstreams((prev) => prev.map((upstream) => upstream.id === id ? result.upstream : upstream));
      setError("");
    } catch (err) {
      setError(formatError(err));
    } finally {
      setCheckingOne(null);
    }
  };

  const handleSpeedTest = async (id: number) => {
    setSpeedTesting(id);
    try {
      const result = await speedTestUpstream(id);
      setUpstreams((prev) => prev.map((upstream) => upstream.id === id ? result.upstream : upstream));
      setError("");
    } catch (err) {
      setError(formatError(err));
    } finally {
      setSpeedTesting(null);
    }
  };

  const handlePrioritySave = async (upstream: Upstream) => {
    const draft = priorityDrafts[upstream.id] ?? String(upstream.priority);
    const priority = Number(draft);
    if (!Number.isFinite(priority)) {
      setError("优先级必须是数字");
      return;
    }
    setSavingPriorityId(upstream.id);
    try {
      const result = await updateUpstream(upstream.id, toUpstreamInput(upstream, { priority }));
      applyUpstreamUpdate(result.upstream);
      setPriorityDrafts((prev) => ({ ...prev, [upstream.id]: String(result.upstream.priority) }));
      setError("");
    } catch (err) {
      setError(formatError(err));
    } finally {
      setSavingPriorityId(null);
    }
  };

  const handleToggleEnabled = async (upstream: Upstream) => {
    setTogglingEnabledId(upstream.id);
    try {
      const result = await updateUpstream(upstream.id, toUpstreamInput(upstream, { enabled: !upstream.enabled }));
      applyUpstreamUpdate(result.upstream);
      setError("");
    } catch (err) {
      setError(formatError(err));
    } finally {
      setTogglingEnabledId(null);
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

  const healthDetail = (upstream: Upstream) => {
    if (!upstream.enabled) return "上游已禁用";
    if (!upstream.healthEnabled) return "健康检查已关闭";
    if (upstream.statusCode > 0) return `HTTP ${upstream.statusCode}`;
    if (upstream.healthStatus === "unknown") return "未检查";
    return "无响应状态码";
  };

  const healthCheckReason = (upstream: Upstream) => {
    if (!upstream.enabled) return "上游已禁用";
    if (!upstream.healthEnabled) return "已关闭健康检查";
    return "";
  };

  const formatLatency = (upstream: Upstream) => {
    if (upstream.healthStatus === "unknown") return "-";
    if (upstream.latencyMs == null) return "-";
    if (upstream.latencyMs < 1000) return `${upstream.latencyMs}ms`;
    return `${(upstream.latencyMs / 1000).toFixed(1)}s`;
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
          <p className="mt-2 text-sm text-muted-foreground">按仓库类型分组管理镜像源上游</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleCheckHealth} disabled={checkingHealth || initialLoading}>
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

      {!initialLoading && upstreams.length > 0 && (
        <section className="mb-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <StatCard label="总上游" value={summary.total} icon={Globe2} tone="bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-400" />
          <StatCard label="已启用" value={summary.enabled} icon={CheckCircle2} tone="bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400" />
          <StatCard label="健康" value={summary.healthy} icon={Activity} tone="bg-violet-50 text-violet-600 dark:bg-violet-500/10 dark:text-violet-400" />
          <StatCard label="异常 / 未检查" value={summary.attention} icon={XCircle} tone="bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400" />
        </section>
      )}

      <Dialog open={dialogOpen} onOpenChange={(open) => {
        setDialogOpen(open);
        if (!open) {
          setEditId(null);
          setForm({ ...emptyInput });
          setFormError("");
        }
      }}>
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
                  onChange={(e) => handleRegistryPrefixChange(e.target.value)}
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
              <Label>测速镜像（可选）</Label>
              <Input
                value={form.speedTestImage}
                onChange={(e) => setForm({ ...form, speedTestImage: e.target.value })}
                placeholder={getDefaultSpeedTestImage(form.registryPrefix)}
              />
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

      <Dialog open={deleteTarget !== null} onOpenChange={(open) => { if (!open && deletingId === null) setDeleteTarget(null); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认删除上游</DialogTitle>
            <DialogDescription>
              {deleteTarget ? `此操作将删除上游“${deleteTarget.name}”且不可恢复，确定继续？` : "此操作将删除该上游且不可恢复，确定继续？"}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)} disabled={deletingId !== null}>取消</Button>
            <Button variant="destructive" onClick={handleDeleteConfirm} disabled={deletingId !== null}>
              {deletingId !== null ? "删除中..." : "确认删除"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {initialLoading ? (
        <Card>
          <CardContent>
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <RefreshCw className="mb-2 h-10 w-10 animate-spin" />
              <p>正在加载上游配置</p>
              <p className="text-sm">请稍候…</p>
            </div>
          </CardContent>
        </Card>
      ) : upstreams.length === 0 ? (
        <Card>
          <CardContent>
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Globe2 className="mb-2 h-10 w-10" />
              <p>暂无上游配置</p>
              <p className="text-sm">点击上方“添加上游”按钮创建第一个上游</p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-6">
          {groupedUpstreams.map((group) => {
            const isOpen = isSectionOpen(group.prefix);
            const enabledCount = group.items.filter((upstream) => upstream.enabled).length;
            const healthyCount = group.items.filter((upstream) => upstream.healthStatus === "healthy").length;
            return (
              <Card key={group.prefix}>
                <CardHeader>
                  <button type="button" className="flex w-full items-center justify-between text-left" onClick={() => toggleSection(group.prefix)}>
                    <div>
                      <div className="flex items-center gap-2">
                        <Badge>{group.prefix}</Badge>
                        <CardTitle className="text-base">{group.prefix}</CardTitle>
                      </div>
                      <p className="mt-2 text-sm text-muted-foreground">{group.items.length} 个上游 · {enabledCount} 已启用 · {healthyCount} 健康</p>
                    </div>
                    <div className="flex items-center gap-1 text-sm text-muted-foreground">
                      <span>{isOpen ? "收起" : "展开"}</span>
                      {isOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                    </div>
                  </button>
                </CardHeader>
                {isOpen && (
                  <CardContent>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>名称</TableHead>
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
                        {group.items.map((upstream) => {
                          const checkReason = healthCheckReason(upstream);
                          const checkDisabled = checkReason !== "";
                          const priorityDraft = priorityDrafts[upstream.id] ?? String(upstream.priority);
                          const priority = Number(priorityDraft);
                          const priorityChanged = priorityDraft !== String(upstream.priority);
                          const priorityInvalid = priorityDraft.trim() === "" || !Number.isFinite(priority);
                          return (
                            <TableRow key={upstream.id}>
                              <TableCell className="font-medium">{upstream.name}</TableCell>
                              <TableCell className="max-w-48 truncate">{upstream.baseUrl}</TableCell>
                              <TableCell>
                                <div className="flex items-center gap-2">
                                  <Input
                                    type="number"
                                    className="h-8 w-20"
                                    value={priorityDraft}
                                    onChange={(e) => setPriorityDrafts((prev) => ({ ...prev, [upstream.id]: e.target.value }))}
                                  />
                                  <Button
                                    variant="outline"
                                    size="sm"
                                    className="h-8"
                                    onClick={() => handlePrioritySave(upstream)}
                                    disabled={savingPriorityId === upstream.id || priorityInvalid || !priorityChanged}
                                  >
                                    {savingPriorityId === upstream.id ? "保存中..." : "保存"}
                                  </Button>
                                </div>
                              </TableCell>
                              <TableCell>
                                <div className="space-y-1">
                                  <div className="flex items-center gap-1">
                                    {healthBadge(upstream.healthStatus)}
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      className="h-6 px-1.5 text-[11px]"
                                      onClick={() => handleCheckOne(upstream.id)}
                                      disabled={checkingOne === upstream.id || checkDisabled}
                                      title={checkingOne === upstream.id ? "检查中..." : (checkReason || "检查健康")}
                                    >
                                      <RefreshCw className={`mr-0.5 h-2.5 w-2.5 ${checkingOne === upstream.id ? "animate-spin" : ""}`} />
                                      检查
                                    </Button>
                                  </div>
                                  <p className="text-xs text-muted-foreground">{healthDetail(upstream)}</p>
                                </div>
                              </TableCell>
                              <TableCell>{formatLatency(upstream)}</TableCell>
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
                                <div className="space-y-1">
                                  <Badge variant={upstream.enabled ? "success" : "warning"}>
                                    {upstream.enabled ? "启用" : "禁用"}
                                  </Badge>
                                  {!upstream.healthEnabled && <p className="text-xs text-muted-foreground">健康检查已关闭</p>}
                                </div>
                              </TableCell>
                              <TableCell className="text-right">
                                <div className="flex justify-end gap-2">
                                  <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => handleToggleEnabled(upstream)}
                                    disabled={togglingEnabledId === upstream.id}
                                  >
                                    {togglingEnabledId === upstream.id ? "切换中..." : (upstream.enabled ? "停用" : "启用")}
                                  </Button>
                                  <Button variant="outline" size="sm" onClick={() => startEdit(upstream)}>编辑</Button>
                                  <Button variant="outline" size="sm" onClick={() => handleDeleteRequest(upstream)} disabled={deletingId === upstream.id}>
                                    <Trash2 className="h-4 w-4" />
                                  </Button>
                                </div>
                              </TableCell>
                            </TableRow>
                          );
                        })}
                      </TableBody>
                    </Table>
                  </CardContent>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </>
  );
}
