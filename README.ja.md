# HermesToken

HermesToken is a private AI API gateway and asset management service.

## Quick Start

```bash
git clone https://github.com/ca0fgh/hermestoken.git
cd hermestoken
docker compose up --build -d
```

The service listens on `http://localhost:3000` by default.

## Development

```bash
make dev-api
make dev-web
```

Backend code lives in the Go project root. The default frontend is in `web/default`, and the classic frontend is in `web/classic`.

## License

See [LICENSE](./LICENSE).
