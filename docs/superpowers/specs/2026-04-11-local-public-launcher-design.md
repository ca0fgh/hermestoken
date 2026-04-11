# Local/Public Startup Launcher Design

## Goal

为 `hermestoken` 提供两套本地启动方式：

- `local` 模式：只启动本地 Docker 服务，访问地址保持 `http://localhost:3000`
- `public` 模式：在本地 Docker 服务之外，额外挂载一个固定公网域名入口，同时保留 `http://localhost:3000` 可访问

目标用户是本地开发和支付联调用例，尤其是需要第三方支付平台回调本机服务的场景。

## Context

当前项目的本地启动方式主要依赖：

- `docker-compose.yml`
- `make start-backend`

但现有流程没有“内网 / 外网”模式区分。项目里影响支付回调和页面跳转的关键值不是容器端口，而是后台配置中的 `ServerAddress`。

同时，固定公网地址不能只靠 Docker 本身实现，必须依赖额外的公网隧道层。用户已经确认接受：

- 使用 `Cloudflare Tunnel`
- 固定域名使用 `pay-local.hermestoken.top`
- 手动在现有 DNS 提供商中配置 DNS 记录

## Requirements

### Functional

1. 提供两个 Python 脚本：
   - `scripts/local.py`
   - `scripts/public.py`
2. `local.py` 负责：
   - 启动 `docker compose up -d`
   - 校验 `http://localhost:3000` 可访问
   - 输出本地访问地址
3. `public.py` 负责：
   - 启动 `docker compose up -d`
   - 校验本地服务可访问
   - 启动预先配置好的 `cloudflared` named tunnel
   - 校验固定公网地址可访问
   - 输出本地地址和公网地址
4. `public` 模式下，本地地址仍然必须可用
5. 固定公网地址使用：
   - `https://pay-local.hermestoken.top`

### Non-Functional

1. 不修改现有 Docker Compose 主结构
2. 不把 `cloudflared` 塞进 Docker sidecar
3. 启动脚本要带明确错误提示
4. 缺少前置条件时快速失败
5. 不自动修改数据库里的 `ServerAddress`

## Out of Scope

以下内容不在这次实现范围内：

- 自动写入后台 `ServerAddress`
- 自动登录 Cloudflare
- 自动创建 named tunnel
- 自动在 DNSOwl 创建 DNS 记录
- 自动生成支付配置
- Windows 平台兼容

这次只做“本地启动编排”和“前置条件校验”。

## Approaches Considered

### Approach A: 两个脚本只做最小启动

内容：

- `local.py` 只执行 `docker compose up -d`
- `public.py` 只执行 `docker compose up -d` 和 `cloudflared tunnel run`

优点：

- 实现最简单
- 改动最少

缺点：

- 缺少健康检查
- 缺少 tunnel / DNS / config 校验
- 出错时排查困难

### Approach B: 两个脚本负责启动 + 校验

内容：

- `local.py`：检查 Docker、启动 compose、等待本地健康检查通过
- `public.py`：检查 Docker、`cloudflared`、config 文件、域名解析，再启动 compose 和 tunnel，并等待本地/公网都通过

优点：

- 最符合支付联调场景
- 错误信息明确
- 使用体验稳定

缺点：

- 逻辑比最小脚本稍复杂

### Approach C: 把 cloudflared 也容器化

内容：

- 在 compose 中增加 `cloudflared` 服务
- Python 脚本只切不同 compose 组合

优点：

- 基础设施更“容器化”

缺点：

- 凭据挂载更复杂
- 调试成本更高
- 对当前目标过重

## Decision

采用 **Approach B**。

原因：

- 它能覆盖“本地内网 / 本地外网”两种模式的真实使用需求
- 它不会入侵现有 compose 架构
- 它能把固定域名外网模式中最容易出问题的环节提前暴露：
  - `cloudflared` 未安装
  - tunnel 配置缺失
  - DNS 未正确指向
  - 本地 3000 服务未就绪

## High-Level Design

### Script 1: `scripts/local.py`

职责：

- 检查 `docker` 和 `docker compose` 是否可用
- 执行 `docker compose up -d`
- 轮询 `http://localhost:3000`
- 成功后输出：
  - `http://localhost:3000`

失败时：

- 如果 Docker 未启动，明确提示
- 如果 compose 启动失败，直接返回错误
- 如果服务健康检查超时，打印最近容器状态

### Script 2: `scripts/public.py`

职责：

- 检查 `docker`
- 检查 `cloudflared`
- 检查本地 tunnel 配置文件
- 检查 `pay-local.hermestoken.top` DNS 是否存在且指向 tunnel 目标
- 执行 `docker compose up -d`
- 启动 named tunnel
- 同时轮询：
  - `http://localhost:3000`
  - `https://pay-local.hermestoken.top`

成功后输出：

- `Local:  http://localhost:3000`
- `Public: https://pay-local.hermestoken.top`

### Tunnel Assumptions

`public.py` 不负责创建 tunnel，只负责运行一个已经准备好的 named tunnel。

预期用户已经准备好：

- named tunnel 名称，例如 `hermestoken-local`
- `~/.cloudflared/config.yml`
- tunnel credentials json
- DNS 记录把 `pay-local.hermestoken.top` 指向 `<tunnel-id>.cfargotunnel.com`

## Configuration Contract

为了避免把实现做死在脚本里，两个脚本需要读取一个轻量配置文件，建议：

- `scripts/launcher_config.json`

建议字段：

- `compose_file`
- `local_url`
- `public_url`
- `cloudflared_tunnel_name`
- `cloudflared_config_path`
- `healthcheck_timeout_seconds`
- `healthcheck_interval_seconds`

这样域名或 tunnel 名变化时，不需要改 Python 代码。

## Error Handling

### `local.py`

应处理：

- `docker` 不存在
- `docker compose` 不存在
- compose 启动失败
- 3000 端口服务未就绪

### `public.py`

额外处理：

- `cloudflared` 不存在
- `~/.cloudflared/config.yml` 不存在
- tunnel 名缺失
- DNS 解析不到 `pay-local.hermestoken.top`
- tunnel 进程异常退出
- 公网地址健康检查失败

## User Experience

### Local Mode

执行：

```bash
python3 scripts/local.py
```

预期输出：

```text
[ok] Docker available
[ok] Containers started
[ok] Local service healthy: http://localhost:3000
```

### Public Mode

执行：

```bash
python3 scripts/public.py
```

预期输出：

```text
[ok] Docker available
[ok] cloudflared available
[ok] DNS configured for pay-local.hermestoken.top
[ok] Containers started
[ok] Tunnel started
[ok] Local service healthy: http://localhost:3000
[ok] Public service healthy: https://pay-local.hermestoken.top
```

## Testing Strategy

### Unit-Level

- 配置文件解析
- 依赖检测函数
- DNS 检查函数
- 健康检查轮询函数
- 子进程命令拼装函数

### Integration-Level

- `local.py` 在 Docker 已安装环境下能成功拉起本地服务
- `public.py` 在 tunnel 配置完备环境下能成功拉起本地服务和公网入口

### Manual Verification

1. 运行 `python3 scripts/local.py`
2. 打开 `http://localhost:3000`
3. 运行 `python3 scripts/public.py`
4. 打开 `http://localhost:3000`
5. 打开 `https://pay-local.hermestoken.top`
6. 确认两者都能访问同一套本地服务

## Files Expected To Change

Create:

- `scripts/local.py`
- `scripts/public.py`
- `scripts/launcher_config.json`
- `docs/payment-local-public-launcher.zh-CN.md`

Possible small update:

- `README.zh_CN.md`

## Risks

1. `public.py` 依赖用户已经完成 Cloudflare named tunnel 配置
2. 域名当前不在 Cloudflare DNS，仍需手动在现有 DNS 服务商中维护 CNAME
3. 如果公网访问需要严格 HTTPS/证书校验，必须依赖正确的 tunnel + DNS 组合
4. 脚本不能代替后台支付配置，只负责启动编排

## Recommendation For Implementation

实现时应优先保证：

1. `local.py` 稳定可用
2. `public.py` 清晰失败
3. 所有前置条件都给出明确提示

不要在第一版里加入：

- 自动写后台配置
- 自动登录 Cloudflare
- 自动创建 tunnel
- 自动改 DNS

