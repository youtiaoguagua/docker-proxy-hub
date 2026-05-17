# Spec: Docker Proxy Hub

## Assumptions to Review

1. This is a new project; the current repository directory is empty, so this spec describes the target product rather than existing code changes.
2. The first milestone is a single-machine web application, not a clustered or multi-node deployment.
3. The backend will be written in Go, the frontend in React, the UI component system will use shadcn/ui with Tailwind CSS and Radix UI primitives, and frontend package management will use pnpm.
4. SQLite is the source of truth for runtime configuration, admin account data, upstream mirror definitions, health results, and request metrics.
5. The application itself exposes both the Web management UI/API and the Docker Registry v2-compatible proxy endpoints.
6. The first version only forwards Docker registry requests; it does not persistently cache manifests or blobs locally.
7. One administrator account is enough for the first version. The first login/setup flow creates it, and the admin can later change the username/password.
8. Supported registry routing is path-prefix based, for example:
   - `domain.com/nginx` routes to Docker Hub.
   - `domain.com/library/nginx` routes to Docker Hub official images.
   - `domain.com/ghcr.io/owner/image:tag` routes to GHCR.
   - `domain.com/gcr.io/project/image:tag` routes to GCR.
   - `domain.com/quay.io/org/image:tag` routes to Quay.
   - `domain.com/registry.k8s.io/component/image:tag` routes to Kubernetes registry.
9. Upstream mirror selection supports configured priority, failover, active health checks, active speed tests, and passive metrics from real pull traffic.
10. HTTP proxy and socket proxy settings are configurable and can be attached to upstream mirror access.

If any assumption above is wrong, update this spec before planning or implementation.

## 1. Objective

Build a Docker image proxy management web application that lets an administrator configure third-party image acceleration sites and expose a unified domain for Docker image pulls.

The product helps users accelerate Docker image downloads by routing image requests through configured upstream mirror providers such as Alibaba Cloud mirrors or other third-party acceleration endpoints. It must support Docker Hub and common non-Docker-Hub registries through path prefixes, while making upstream availability, latency, failover, and usage visible in a Web UI.

Primary users:

- Server administrators who want a self-hosted image proxy entrypoint.
- Developers or internal teams pulling Docker images from Docker Hub, GHCR, GCR, Quay, and Kubernetes registries.

Success means an admin can deploy the app, create the initial admin account, configure upstream mirrors and proxy settings, then pull images through one domain using documented path formats.

## 2. Commands

Because the repository is currently empty, these commands define the expected project interface after implementation.

```bash
# Backend development
go run ./cmd/server

# Backend tests
go test ./...

# Backend formatting
gofmt -w .

# Frontend install
pnpm install

# Initialize shadcn/ui in the Vite React frontend
pnpm --dir web dlx shadcn@latest init -t vite

# Add shadcn/ui components as needed
pnpm --dir web dlx shadcn@latest add button card table form input select badge dialog sheet dropdown-menu tabs chart sidebar

# Frontend development
pnpm --dir web dev

# Frontend build
pnpm --dir web build

# Frontend lint/type check
pnpm --dir web lint
pnpm --dir web typecheck

# Full production build
pnpm --dir web build && go build -o bin/docker-proxy-hub ./cmd/server
```

If the final project structure differs, update this section before implementation.

## 3. Project Structure

Target structure:

```text
cmd/server/                 → Go application entrypoint
internal/api/               → Admin REST API and Docker Registry proxy HTTP handlers
internal/auth/              → Initial setup, admin login, password update, session handling
internal/config/            → Runtime configuration loading and validation
internal/db/                → SQLite migrations, queries, repository layer
internal/health/            → Active health checks and speed tests for upstream mirrors
internal/proxy/             → Registry routing, upstream selection, failover, request forwarding
internal/registry/          → Registry path parsing and Docker Registry v2 semantics
internal/metrics/           → Passive request metrics and upstream result recording
web/                        → React frontend managed by pnpm
web/src/                    → React application source
web/src/pages/              → Admin pages: setup/login, upstreams, proxy config, monitoring, usage docs
web/src/components/         → Shared React UI components
web/src/components/ui/      → shadcn/ui generated components owned by the project
web/src/api/                → Frontend API client code
web/src/lib/utils.ts        → Shared frontend utilities, including shadcn/ui `cn` helper
spec/                       → Product and implementation specifications
```

## 4. Code Style

### Go style

Use small packages grouped by responsibility. Keep HTTP boundary parsing separate from business logic. Validate untrusted input at API boundaries and store typed configuration in SQLite.

Example style:

```go
type Upstream struct {
    ID              int64
    RegistryPrefix  string
    Name            string
    BaseURL         string
    Priority        int
    Enabled         bool
    ProxyID         *int64
}

type UpstreamSelector interface {
    Select(ctx context.Context, registryPrefix string) ([]Upstream, error)
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    target, err := h.parser.Parse(r.URL.Path)
    if err != nil {
        http.Error(w, "invalid image path", http.StatusBadRequest)
        return
    }

    h.forwarder.Forward(w, r, target)
}
```

### React style

Use React with TypeScript, Tailwind CSS, and shadcn/ui components. shadcn/ui components are copied into `web/src/components/ui/` and treated as project-owned UI primitives. Use Lucide icons for navigation and status indicators. Keep page-level data loading in pages, typed API models in `web/src/api/`, and reusable layout/form/table pieces in components.

The visual direction should follow the provided CodexProxy dashboard reference:

- Fixed left sidebar with product logo/name, version badge, primary navigation, online status, and footer actions.
- Light theme first, with rounded cards, subtle borders, muted gray page background, blue selected navigation state, and compact spacing.
- Dashboard overview cards across the top for totals, available upstreams, abnormal upstreams, and today's requests.
- Usage/statistics section with metric tiles, colored icon blocks, and request/latency trend charts.
- Management pages should reuse the same shell and shadcn/ui `Card`, `Table`, `Form`, `Input`, `Select`, `Badge`, `Button`, `Dialog`, `Sheet`, `Tabs`, `Sidebar`, and `Chart` patterns.
- Responsive behavior should collapse the sidebar into a sheet/drawer on narrow screens.

Example style:

```tsx
import { Activity, CheckCircle2, Server, XCircle } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

type StatCardProps = {
  title: string;
  value: string;
  tone: "blue" | "green" | "red";
};

export function StatCard({ title, value, tone }: StatCardProps) {
  const Icon = tone === "green" ? CheckCircle2 : tone === "red" ? XCircle : Server;

  return (
    <Card className="shadow-sm">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
        <div className="rounded-xl bg-muted p-3">
          <Icon className="h-5 w-5" />
        </div>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-semibold tracking-tight">{value}</div>
      </CardContent>
    </Card>
  );
}
```

## 5. Functional Requirements

### 5.1 Initial setup and admin authentication

- On first visit, if no admin exists, show setup flow.
- Setup creates a single administrator account with username and password.
- Store password using a modern password hash suitable for server-side authentication.
- After setup, require login for all Web management pages and admin APIs.
- Provide a page to change administrator username and password.
- Do not require multi-user roles, password reset email, OIDC, or OAuth2 in the first version.

### 5.2 Registry path routing

The proxy must support path-prefix routing:

| Incoming path | Registry target |
| --- | --- |
| `/v2/nginx/...` or image `domain.com/nginx:tag` | Docker Hub default namespace behavior |
| `/v2/library/nginx/...` | Docker Hub official image namespace |
| `/v2/ghcr.io/owner/image/...` | GitHub Container Registry |
| `/v2/gcr.io/project/image/...` | Google Container Registry |
| `/v2/quay.io/org/image/...` | Quay |
| `/v2/registry.k8s.io/name/...` | Kubernetes registry |
| Custom configured prefix | Custom registry/upstream mapping |

The implementation must preserve Docker Registry v2 request behavior for manifests, blobs, tags, and authentication challenge flows as needed for `docker pull` compatibility.

### 5.3 Upstream mirror management

The admin UI and API must allow creating, editing, enabling/disabling, and deleting upstream mirror entries.

Each upstream mirror should include:

- Display name.
- Registry prefix or registry type.
- Base URL of the third-party mirror or upstream endpoint.
- Priority order.
- Enabled/disabled status.
- Optional linked HTTP or socket proxy configuration.
- Health check configuration.
- Speed test configuration.

### 5.4 Upstream selection, failover, health checks, and speed tests

- For a given registry prefix, enabled upstreams are considered in priority order.
- Unhealthy upstreams should be skipped when healthy alternatives exist.
- If the selected upstream fails during a pull request, the proxy should try the next eligible upstream when the request can be safely retried.
- Active health checks run on a schedule and record success/failure, status code, error, and latency.
- Active speed tests run on a schedule using a configured lightweight test target and record measured latency/throughput.
- Passive metrics from real pull traffic record upstream used, request duration, status, failure reason, and bytes transferred when available.
- The UI must show current health, recent speed test results, and recent failover/request events.

### 5.5 HTTP and socket proxy configuration

The admin UI and API must allow defining proxy settings that can be attached to upstream mirrors.

Required proxy types:

- HTTP/HTTPS proxy URL.
- Socket proxy setting.

The spec intentionally leaves socket proxy details open until implementation planning, because this can mean Unix socket, Docker socket, SOCKS proxy, or another socket transport depending on the intended deployment environment.

### 5.6 Web UI layout and navigation

The admin frontend must use shadcn/ui as the component foundation and follow the provided dashboard reference style.

Required navigation items:

- 仪表盘: system overview, upstream totals, health status, today's requests, request trend, latency trend.
- 上游管理: Docker Hub/GHCR/GCR/Quay/K8s/custom upstream mirror configuration.
- 代理配置: HTTP and socket proxy settings.
- 监控日志: health checks, speed tests, request logs, upstream errors, failover events.
- 系统设置: admin account update and runtime settings.
- 使用文档: copyable docker pull examples and routing explanation.

The layout must include:

- Persistent left sidebar on desktop.
- Collapsible mobile sidebar using a shadcn/ui sheet/drawer pattern.
- Top page header with title, description, and contextual actions such as refresh or create.
- Reusable dashboard/stat cards, data tables, filter controls, dialogs/sheets for create/edit forms, and chart panels.

### 5.7 Monitoring and logs

The UI must include monitoring/log pages for:

- Upstream health status.
- Speed test history.
- Recent image pull requests.
- Recent upstream errors.
- Failover events.

Logs shown in the UI should be operationally useful without exposing stored admin passwords or sensitive proxy credentials.

### 5.8 Usage documentation page

The Web UI must include a usage page showing copyable examples for:

```bash
docker pull domain.com/nginx:latest
docker pull domain.com/library/nginx:latest
docker pull domain.com/ghcr.io/owner/image:latest
docker pull domain.com/gcr.io/project/image:latest
docker pull domain.com/quay.io/org/image:latest
docker pull domain.com/registry.k8s.io/pause:latest
```

The page should explain that Docker Hub can use the short path while non-Docker-Hub registries are distinguished by registry prefix.

## 6. Testing Strategy

### Backend tests

Use Go tests for:

- Registry path parsing.
- Upstream selection by priority and health.
- Failover behavior.
- Admin setup/login/password change logic.
- SQLite repository behavior.
- HTTP handler behavior for admin APIs.

### Frontend tests

Use the frontend test stack selected during implementation planning. At minimum verify:

- Initial setup flow renders when no admin exists.
- Login is required after setup.
- Upstream form validates required fields.
- Proxy configuration form supports HTTP and socket proxy types.
- Dashboard shell renders the shadcn/ui-based sidebar, header, stat cards, and chart panels.
- Monitoring page renders health and speed data.
- Usage page shows correct Docker pull examples.

### Integration/manual verification

Before marking implementation done:

- Run `go test ./...`.
- Run frontend lint/type check/build commands.
- Start the app locally.
- Verify the UI visually follows the provided CodexProxy-style dashboard reference: left sidebar, light cards, metric tiles, charts, and responsive navigation.
- Complete first-admin setup in browser.
- Configure at least one Docker Hub mirror.
- Pull a test image through the local proxy domain or local registry endpoint.
- Verify health checks and request logs appear in the UI.

## 7. Boundaries

### Always

- Use shadcn/ui and Tailwind CSS for frontend UI primitives and styling.
- Keep the desktop UI aligned with the provided CodexProxy-style reference unless the spec is updated.
- Validate all admin API inputs at the HTTP boundary.
- Hash admin passwords; never store plaintext passwords.
- Treat upstream mirror URLs and proxy credentials as sensitive configuration.
- Keep Docker Registry proxy endpoints accessible to Docker clients without requiring Web admin session cookies.
- Require admin authentication for Web UI and admin APIs after setup.
- Record enough health and request data to diagnose upstream failure and failover.
- Keep path-prefix behavior documented in the UI.

### Ask first

- Adding dependencies beyond common Go/React build requirements.
- Introducing a local image cache for manifests or blobs.
- Supporting clustered deployments or external databases.
- Adding OIDC/OAuth2, multi-user roles, or public user registration.
- Changing the path format for non-Docker-Hub registries.
- Treating socket proxy as a specific protocol if not yet confirmed.

### Never

- Replacing shadcn/ui with another UI framework without explicit approval.
- Store admin passwords or proxy secrets in plaintext.
- Expose sensitive proxy credentials in logs or the monitoring UI.
- Delete failing tests to make verification pass.
- Implement destructive cleanup of image data in the first version, because local image caching is out of scope.
- Require users to rewrite image names in a way that conflicts with the agreed path-prefix model.

## 8. Success Criteria

The feature is done when all of the following are true:

1. A fresh deployment opens a first-admin setup page.
2. After setup, admin UI and admin APIs require login.
3. The administrator can change the admin username and password.
4. The administrator can configure upstream mirrors for Docker Hub, GHCR, GCR, Quay, Kubernetes registry, and custom registry prefixes.
5. The administrator can configure HTTP proxy and socket proxy entries and attach them to upstream mirrors.
6. Pulling Docker Hub images through `domain.com/nginx` or `domain.com/library/nginx` routes to the configured Docker Hub mirror.
7. Pulling GHCR/GCR/Quay/K8s images through path prefixes such as `domain.com/ghcr.io/...` routes to the matching configured upstream.
8. Upstream selection respects priority, health state, and failover behavior.
9. Active health checks and speed tests are recorded and visible in the UI.
10. Real pull requests record passive metrics visible in the monitoring/log pages.
11. The UI uses shadcn/ui components and visually follows the provided CodexProxy-style dashboard reference for sidebar navigation, overview cards, metric tiles, charts, and management tables.
12. The UI includes copyable Docker pull examples for all supported registry categories.
13. Backend tests, frontend checks, production build, and at least one manual pull-through test pass.

## 9. Open Questions

1. What exact meaning should “socket proxy” have for this project: Unix socket HTTP transport, Docker socket integration, SOCKS proxy, or another socket-based proxy mechanism?
2. Should Docker Registry proxy endpoints be public by default, protected by registry auth, or expected to sit behind an external network boundary?
3. Which third-party mirror endpoints should be provided as built-in presets, if any?
4. What default health-check URL or test image should be used for Docker Hub, GHCR, GCR, Quay, and Kubernetes registry?
5. How long should request logs, health results, and speed test history be retained in SQLite?
6. Should the app expose Prometheus metrics in addition to the Web monitoring pages?

## 10. Review Gate

This specification is ready for human review. Do not move to implementation planning until the assumptions, requirements, success criteria, and open questions are reviewed and either approved or revised.
