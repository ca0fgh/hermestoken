# HermesToken

HermesToken est un service prive de passerelle API IA et de gestion des actifs.

## Demarrage Rapide

```bash
git clone https://github.com/ca0fgh/hermestoken.git
cd hermestoken
docker compose up --build -d
```

Le service ecoute `http://localhost:3000` par defaut.

## Developpement

```bash
make dev-api
make dev-web
```

Le backend se trouve a la racine du projet Go. Le frontend classique dans `web/classic` est le frontend par defaut pour `make dev-web` et `make build-frontend`; le frontend de compatibilite reste dans `web/default` et peut etre lance avec `make dev-web-default`.

## Licence

Voir [LICENSE](./LICENSE).
