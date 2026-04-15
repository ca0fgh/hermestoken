# 支付服务本地/公网启动器说明

适用脚本：

- `scripts/local.py`
- `scripts/public.py`
- `scripts/launcher_config.json`

说明：

- 两个脚本都会先读取 `scripts/launcher_config.json`
- 两个脚本都会先在宿主机构建 `web/dist`，然后再让 Docker 只打包预构建前端产物，避免容器内 `vite build` OOM
- 如果这个文件不存在、JSON 非法、不是顶层对象、缺少必填字段、必填字符串为空，或者超时/间隔字段非法，脚本会在启动前直接失败

## 0. 启动配置文件

启动前，请先确认 `scripts/launcher_config.json` 存在，且至少包含这些字段：

- `compose_file`
- `local_url`
- `public_url`
- `cloudflared_tunnel_name`
- `cloudflared_config_path`
- `healthcheck_timeout_seconds`
- `healthcheck_interval_seconds`

当前默认内容示例：

```json
{
  "compose_file": "docker-compose.yml",
  "local_url": "http://localhost:3000",
  "public_url": "https://pay-local.hermestoken.top",
  "cloudflared_tunnel_name": "hermestoken-local",
  "cloudflared_config_path": "~/.cloudflared/config.yml",
  "healthcheck_timeout_seconds": 60,
  "healthcheck_interval_seconds": 1
}
```

字段合法性要求：

- 配置文件必须是一个顶层 JSON 对象
- `compose_file`、`local_url`、`public_url`、`cloudflared_tunnel_name`、`cloudflared_config_path` 必须是非空字符串
- `public_url` 必须是一个能解析出 hostname 的有效 URL，建议直接使用 `https://pay-local.hermestoken.top`
- `healthcheck_timeout_seconds` 必须是数字且大于 `0`
- `healthcheck_interval_seconds` 必须是数字且大于等于 `0`

## 1. 前置条件

### 1.1 `local.py`（本地模式）

- 已安装并可用 `docker`
- `docker compose version` 可正常执行
- 已安装并可用 `bun`
- `launcher_config.json` 中的 `compose_file` 路径必须存在并可访问

补充：

- `compose_file` 可以是相对路径，也可以是绝对路径
- 如果使用相对路径，脚本会按项目根目录解析

### 1.2 `public.py`（公网模式）

除本地模式前置条件外，还需要：

- 已在 Cloudflare 中创建命名 tunnel（默认名见 `cloudflared_tunnel_name`）
- 如果使用本地管理的 tunnel 配置模式，本机存在 `~/.cloudflared/config.yml`（或 `launcher_config.json` 中配置的等效路径）
- 如果使用 token 模式，需要在 `launcher_config.json` 中配置 `cloudflared_tunnel_token` 或 `cloudflared_tunnel_token_path`
- 公网脚本会把 `cloudflared` 作为 Docker 容器启动，并与 `new-api` 共享网络命名空间

注意：

- 公网模式不会自动创建 tunnel，也不会自动创建或修改 DNS 记录。
- 本地管理配置模式下，脚本仍会校验公网 hostname 的 CNAME 是否指向 `*.cfargotunnel.com`。

## 2. 启动方式

在项目根目录执行：

### 2.1 本地模式

```bash
python3 scripts/local.py
python3 scripts/local.py update
```

说明：

- 启动前会先在宿主机执行前端构建，再执行 `docker compose up -d --build`。
- `deploy` 和 `update` 都会重建镜像并后台拉起容器，区别只体现在输出标签。

### 2.2 公网模式

```bash
python3 scripts/public.py
python3 scripts/public.py update
```

说明：

- 会先委托 `scripts/local.py` 启动本地服务，因此同样会先做宿主机前端构建，再重建镜像并启动容器。
- 本地服务健康后，会再启动一个 Docker 化的 `cloudflared` 容器，而不是在宿主机直接拉起 `cloudflared` 进程。

## 3. 成功输出示例

### 3.1 本地模式成功

```text
[ok] Docker available
[info] Building frontend on host before docker packaging (WEB_DIST_STRATEGY=prebuilt)...
[ok] Containers started
[ok] Local deploy healthy: http://localhost:3000
```

### 3.2 公网模式成功

```text
[ok] DNS configured for pay-local.hermestoken.top
[ok] Docker available
[info] Building frontend on host before docker packaging (WEB_DIST_STRATEGY=prebuilt)...
[ok] Containers started
[ok] Local deploy healthy: http://localhost:3000
[ok] Tunnel container started: hermestoken-public-cloudflared
[ok] Public deploy healthy: https://pay-local.hermestoken.top
[ok] Local URL:  http://localhost:3000
[ok] Public URL: https://pay-local.hermestoken.top
```

说明：

- 公网模式成功后，本地地址 `http://localhost:3000` 仍然可用。
- 成功路径下 `cloudflared` 容器会保持后台运行，不会被脚本主动停止。

## 4. 常见失败信息与处理

所有错误都会以 `[error]` 开头，并附带 `Next step` 提示。常见情况如下：

- `Missing required executable: docker`
  - 含义：未安装 Docker 或未加入 PATH。
  - 处理：安装 Docker Desktop/Engine，并确认 `docker` 命令可执行。

- `` `docker compose` is unavailable. ``
  - 含义：Compose v2 不可用。
  - 处理：升级/修复 Docker Compose，确保 `docker compose version` 成功。

- `Missing required executable: bun`
  - 含义：脚本无法在宿主机构建前端。
  - 处理：安装 Bun，并确认 `bun --version` 成功。

- `cloudflared config file not found: ...`
  - 含义：`cloudflared_config_path` 指向的配置文件不存在。
  - 处理：创建/修正 `~/.cloudflared/config.yml`（或更新 `launcher_config.json` 路径）。

- `No CNAME record found for pay-local.hermestoken.top`
  - 含义：域名没有 CNAME 记录。
  - 处理：添加 CNAME 到 tunnel 的 `*.cfargotunnel.com` 目标。

- `CNAME target for pay-local.hermestoken.top is ... expected a .cfargotunnel.com target`
  - 含义：CNAME 指向错误目标。
  - 处理：改为 Cloudflare Tunnel 分配的 `*.cfargotunnel.com`。

- `Health check timed out for http://localhost:3000` 或 `Health check timed out for https://pay-local.hermestoken.top`
  - 含义：本地服务或公网地址在超时时间内未变为健康。
  - 处理：检查容器日志、端口占用、应用启动耗时、tunnel 连通性；必要时调大 `healthcheck_timeout_seconds`。
  - 补充：`local.py` 在本地健康检查失败时，还会先打印 `[info] Recent container status (docker compose ps):` 作为额外诊断信息。

- `Tunnel container entered ... before https://... became healthy`
  - 含义：Docker 化 `cloudflared` 容器提前退出或异常停止。
  - 处理：检查错误里附带的 cloudflared 容器日志，并核对 token / `cloudflared_config_path` / tunnel 配置。
