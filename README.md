# docker-proxy-hub

一个自托管的 Docker 镜像代理与上游管理面板。

它把多个镜像源统一收敛到一个入口，提供：
- 多上游管理与优先级切换
- 健康检查与测速
- 请求日志与监控页面
- 内置 Web 管理界面
- 单二进制部署与容器化发布

## 路径规则

- Docker Hub 短路径：`domain.com/nginx:latest`
- Docker Hub 官方镜像：`domain.com/library/nginx:latest`
- GHCR：`domain.com/ghcr.io/owner/image:tag`
- GCR：`domain.com/gcr.io/project/image:tag`
- Quay：`domain.com/quay.io/org/image:tag`
- Kubernetes Registry：`domain.com/registry.k8s.io/pause:3.9`

项目内置页面里也有相同的使用示例：`/docs`。

## 本地开发

### 依赖

- Node.js 22+
- pnpm
- Go 1.25+

### 安装依赖

```bash
pnpm install
```

### 启动前端开发服务器

```bash
pnpm dev:web
```

默认地址：`http://127.0.0.1:5173`

### 启动后端

首次启动后端前，先生成一次前端产物并同步到 Go embed 目录：

```bash
pnpm build:web
pnpm dev:server
```

后端默认监听：`http://127.0.0.1:8080`

首次启动后访问 `/setup` 完成管理员初始化。

## 构建

### 前端构建并同步嵌入产物

```bash
pnpm build:web
```

这个命令会：
1. 构建 `web/dist`
2. 同步产物到 `internal/frontend/dist`

### 构建服务端二进制

```bash
go test ./...
pnpm build:server
```

### 一次性构建完整发布产物

```bash
pnpm build
```

## Docker

### 本地构建镜像

```bash
docker build -t docker-proxy-hub .
```

### 运行容器

```bash
docker run -d \
  --name docker-proxy-hub \
  -p 8080:8080 \
  -v docker-proxy-hub-data:/data \
  -e DPH_JWT_SECRET=change-me \
  docker-proxy-hub
```

容器内默认：
- 监听地址：`DPH_ADDR=:8080`
- 数据库路径：`DPH_DB_PATH=/data/docker-proxy-hub.db`

## 配置

程序会按下面的顺序读取配置：
1. 环境变量
2. 当前目录下的 `config.yaml`
3. `/etc/docker-proxy-hub/config.yaml`

常用环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DPH_ADDR` | `:8080` | 服务监听地址 |
| `DPH_DB_PATH` | `./docker-proxy-hub.db` | SQLite 数据库路径 |
| `DPH_JWT_SECRET` | 空 | 登录签名密钥，生产环境必须设置 |
| `DPH_JWT_TTL` | `24h` | 登录有效期 |
| `DPH_COOKIE_NAME` | `dph_token` | 登录 Cookie 名称 |
| `DPH_HEALTH_CHECK_INTERVAL` | `30s` | 健康检查周期 |
| `DPH_LOG_LEVEL` | `info` | `debug/info/warn/error` |
| `DPH_LOG_FORMAT` | `json` | `json` 或 `text` |

示例 `config.yaml`：

```yaml
addr: ":8080"
db_path: "./docker-proxy-hub.db"
jwt_secret: "change-me"
jwt_ttl: "24h"
cookie_name: "dph_token"
health_check_interval: "30s"
log_level: "info"
log_format: "text"
```

## GHCR 自动发布

仓库包含 GitHub Actions workflow：
- `push` 到 `main` / `master` 时自动构建并推送镜像
- 推送 `v*` tag 时自动打 tag 镜像
- `pull_request` 只验证构建，不推送

默认镜像名：

```text
ghcr.io/<owner>/<repo>
```

确保仓库 Actions 具备 `packages: write` 权限即可直接推送到 GHCR。
