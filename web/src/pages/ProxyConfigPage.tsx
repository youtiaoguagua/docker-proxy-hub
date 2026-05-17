import { useCallback, useEffect, useRef, useState } from "react";
import { Settings } from "lucide-react";

import { listUpstreams } from "@/api/client";
import type { Upstream } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function ProxyConfigPage() {
  const [upstreams, setUpstreams] = useState<Upstream[]>([]);
  const [error, setError] = useState("");
  const loadedRef = useRef(false);

  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;
    listUpstreams()
      .then((result) => { setUpstreams(result.upstreams ?? []); })
      .catch(() => { setError("加载上游列表失败"); });
  }, []);

  const refresh = useCallback(() => {
    listUpstreams()
      .then((result) => { setUpstreams(result.upstreams ?? []); })
      .catch(() => { setError("加载上游列表失败"); });
  }, []);

  if (error) {
    return (
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">代理配置</h1>
        <p className="mt-2 text-sm text-destructive">{error}</p>
        <button className="mt-4 text-sm text-primary hover:underline" onClick={refresh}>重试</button>
      </div>
    );
  }

  const proxied = upstreams.filter((u) => u.httpProxy);
  const unproxied = upstreams.filter((u) => !u.httpProxy);

  return (
    <>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">代理配置</h1>
        <p className="mt-2 text-sm text-muted-foreground">为上游镜像配置 HTTP 代理</p>
      </div>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Settings className="h-5 w-5" />
            配置说明
          </CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>为每个上游镜像配置 HTTP 代理，用于通过代理服务器访问上游镜像源。</p>
          <p>在「上游管理」页面编辑上游时，可以填写 HTTP 代理地址（如 <code className="rounded bg-muted px-1 py-0.5 text-xs">http://proxy:8080</code>）。</p>
          <p>留空表示该上游直连，不使用代理。</p>
        </CardContent>
      </Card>

      {proxied.length > 0 && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>已配置代理的上游</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>仓库类型</TableHead>
                  <TableHead>上游地址</TableHead>
                  <TableHead>代理地址</TableHead>
                  <TableHead>状态</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {proxied.map((upstream) => (
                  <TableRow key={upstream.id}>
                    <TableCell className="font-medium">{upstream.name}</TableCell>
                    <TableCell><Badge>{upstream.registryPrefix}</Badge></TableCell>
                    <TableCell className="max-w-48 truncate">{upstream.baseUrl}</TableCell>
                    <TableCell className="font-mono text-xs max-w-48 truncate">{upstream.httpProxy}</TableCell>
                    <TableCell>
                      <Badge variant={upstream.enabled ? "success" : "warning"}>
                        {upstream.enabled ? "启用" : "禁用"}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {unproxied.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>直连（无代理）</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>仓库类型</TableHead>
                  <TableHead>上游地址</TableHead>
                  <TableHead>状态</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {unproxied.map((upstream) => (
                  <TableRow key={upstream.id}>
                    <TableCell className="font-medium">{upstream.name}</TableCell>
                    <TableCell><Badge>{upstream.registryPrefix}</Badge></TableCell>
                    <TableCell className="max-w-48 truncate">{upstream.baseUrl}</TableCell>
                    <TableCell>
                      <Badge variant={upstream.enabled ? "success" : "warning"}>
                        {upstream.enabled ? "启用" : "禁用"}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {upstreams.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
            <Settings className="mb-2 h-10 w-10" />
            <p>暂无上游配置</p>
            <p className="text-sm">请先在「上游管理」页面添加上游</p>
          </CardContent>
        </Card>
      )}
    </>
  );
}