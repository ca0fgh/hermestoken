# HermesToken

HermesToken 是一個私有 AI API 閘道和資產管理服務。

## 快速開始

```bash
git clone https://github.com/ca0fgh/hermestoken.git
cd hermestoken
docker compose up --build -d
```

預設訪問地址：`http://localhost:3000`。

## 本地開發

```bash
make dev-api
make dev-web
```

後端程式碼位於 Go 專案根目錄；經典前端 `web/classic` 現在是 `make dev-web` 和 `make build-frontend` 的預設入口；相容前端保留在 `web/default`，可用 `make dev-web-default` 啟動。

## 授權

見 [LICENSE](./LICENSE)。
