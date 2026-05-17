import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const examples = [
  { label: "Docker Hub", command: "docker pull domain.com/nginx:latest" },
  { label: "Docker Hub (官方)", command: "docker pull domain.com/library/nginx:latest" },
  { label: "GHCR", command: "docker pull domain.com/ghcr.io/owner/image:latest" },
  { label: "GCR", command: "docker pull domain.com/gcr.io/project/image:latest" },
  { label: "Quay", command: "docker pull domain.com/quay.io/org/image:latest" },
  { label: "K8s", command: "docker pull domain.com/registry.k8s.io/pause:3.9" },
];

export function UsageDocsPage() {
  const copy = (text: string) => {
    navigator.clipboard.writeText(text).catch(() => {});
  };

  return (
    <>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">使用文档</h1>
        <p className="mt-2 text-sm text-muted-foreground">Docker 镜像代理使用说明与示例</p>
      </div>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle>路径规则</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-muted-foreground">
          <p>Docker Hub 镜像使用短路径：<code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">domain.com/nginx</code></p>
          <p>非 Docker Hub 仓库通过路径前缀区分：<code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">domain.com/ghcr.io/...</code></p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>拉取示例</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {examples.map((example) => (
            <div key={example.label} className="flex items-center justify-between rounded-lg border p-3">
              <div>
                <span className="mr-2 inline-flex rounded-md bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">{example.label}</span>
                <code className="text-sm font-mono">{example.command}</code>
              </div>
              <button
                className="rounded-md bg-muted px-3 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
                onClick={() => copy(example.command)}
              >
                复制
              </button>
            </div>
          ))}
        </CardContent>
      </Card>
    </>
  );
}