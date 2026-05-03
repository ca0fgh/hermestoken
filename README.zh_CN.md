# HermesToken

HermesToken 是一个私有 AI API 网关和资产管理服务。

## 快速开始

```bash
git clone https://github.com/ca0fgh/hermestoken.git
cd hermestoken
docker compose up --build -d
```

默认访问地址：`http://localhost:3000`。

## 本地开发

```bash
make dev-api
make dev-web
```

后端代码位于 Go 项目根目录；经典前端 `web/classic` 现在是 `make dev-web` 和 `make build-frontend` 的默认入口；兼容前端保留在 `web/default`，可用 `make dev-web-default` 启动。

## 许可证

见 [LICENSE](./LICENSE)。
