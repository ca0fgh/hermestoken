# 支付服务本地/公网启动器说明

适用脚本：

- `scripts/local.py`
- `scripts/public.py`
- `scripts/launcher_config.json`

说明：

- 两个脚本都会先读取 `scripts/launcher_config.json`
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
- `launcher_config.json` 中的 `compose_file` 路径必须存在并可访问

补充：

- `compose_file` 可以是相对路径，也可以是绝对路径
- 如果使用相对路径，脚本会按项目根目录解析

### 1.2 `public.py`（公网模式）

除本地模式前置条件外，还需要：

- 已安装并可用 `cloudflared`
- 已安装并可用 `nslookup`，因为启动前会用它校验 `pay-local.hermestoken.top` 的 CNAME
- 已在 Cloudflare 中创建命名 tunnel（默认名见 `cloudflared_tunnel_name`）
- 本机存在 `~/.cloudflared/config.yml`（或 `launcher_config.json` 中配置的等效路径）
- `cloudflared` 配置文件内容必须和 `launcher_config.json` 中的 `cloudflared_tunnel_name`、凭据文件、ingress 转发目标一致
- `pay-local.hermestoken.top` 已配置 CNAME，且目标是 `*.cfargotunnel.com`

注意：

- 公网模式会校验 DNS 与 tunnel 可运行性，但不会自动创建 tunnel。
- 公网模式不会自动创建或修改 DNS 记录。

## 2. 启动方式

在项目根目录执行：

### 2.1 本地模式

```bash
python3 scripts/local.py
```

说明：

- 默认执行 `docker compose up -d --build`，每次启动都会先重建镜像再后台拉起容器。

### 2.2 公网模式

```bash
python3 scripts/public.py
```

说明：

- 会先委托 `scripts/local.py` 启动本地服务，因此同样默认执行 `docker compose up -d --build`，先重建镜像再启动容器。

## 3. 成功输出示例

### 3.1 本地模式成功

```text
[ok] Docker available
[ok] Containers started
[ok] Local service healthy: http://localhost:3000
```

### 3.2 公网模式成功

```text
[ok] cloudflared available
[ok] DNS configured for pay-local.hermestoken.top
[ok] Docker available
[ok] Containers started
[ok] Local service healthy: http://localhost:3000
[ok] Tunnel started: hermestoken-local
[ok] Public service healthy: https://pay-local.hermestoken.top
[ok] Local URL:  http://localhost:3000
[ok] Public URL: https://pay-local.hermestoken.top
```

说明：

- 公网模式成功后，本地地址 `http://localhost:3000` 仍然可用。
- 成功路径下 tunnel 进程会保持后台运行，不会被脚本主动停止。

## 4. 常见失败信息与处理

所有错误都会以 `[error]` 开头，并附带 `Next step` 提示。常见情况如下：

- `Missing required executable: docker`
  - 含义：未安装 Docker 或未加入 PATH。
  - 处理：安装 Docker Desktop/Engine，并确认 `docker` 命令可执行。

- `` `docker compose` is unavailable. ``
  - 含义：Compose v2 不可用。
  - 处理：升级/修复 Docker Compose，确保 `docker compose version` 成功。

- `Missing required executable: cloudflared`
  - 含义：公网模式缺少 cloudflared。
  - 处理：安装 cloudflared，并确认 `cloudflared --version` 成功。

- `cloudflared config file not found: ...`
  - 含义：`cloudflared_config_path` 指向的配置文件不存在。
  - 处理：创建/修正 `~/.cloudflared/config.yml`（或更新 `launcher_config.json` 路径）。

- `No CNAME record found for pay-local.hermestoken.top`
  - 含义：域名没有 CNAME 记录。
  - 处理：添加 CNAME 到 tunnel 的 `*.cfargotunnel.com` 目标。

- `CNAME target for pay-local.hermestoken.top is ... expected a .cfargotunnel.com target`
  - 含义：CNAME 指向错误目标。
  - 处理：改为 Cloudflare Tunnel 分配的 `*.cfargotunnel.com`。

- `Unable to validate DNS CNAME because nslookup is unavailable`
  - 含义：本机缺少 `nslookup`，脚本无法校验公网域名。
  - 处理：安装提供 `nslookup` 的 DNS 工具，再重新执行。

- `DNS lookup timed out for pay-local.hermestoken.top`
  - 含义：本地 DNS 解析超时，可能是网络或解析器问题。
  - 处理：检查本机网络、DNS 解析配置，确认该域名可以在本机正常解析。

- `Failed to resolve CNAME for pay-local.hermestoken.top`
  - 含义：本机 DNS 查询失败，可能是域名未生效或解析器无法访问。
  - 处理：检查 DNS 记录是否已生效，并重新验证 `nslookup -type=CNAME pay-local.hermestoken.top`。

- `Health check timed out for http://localhost:3000` 或 `Health check timed out for https://pay-local.hermestoken.top`
  - 含义：本地服务或公网地址在超时时间内未变为健康。
  - 处理：检查容器日志、端口占用、应用启动耗时、tunnel 连通性；必要时调大 `healthcheck_timeout_seconds`。
  - 补充：`local.py` 在本地健康检查失败时，还会先打印 `[info] Recent container status (docker compose ps):` 作为额外诊断信息。

- `Tunnel process exited unexpectedly with exit code ...`
  - 含义：`cloudflared tunnel run` 提前退出。
  - 处理：检查 `~/.cloudflared/config.yml` 的 tunnel 名称、凭据文件与 ingress 配置是否匹配，并确认与 `launcher_config.json` 中的 `cloudflared_tunnel_name` 一致。
