# AGENTS.md — Classic Frontend Conventions

## Role

`web/classic/` is the primary frontend for current HermesToken product work.

Use this UI for marketplace, token-management, preview, screenshot, and acceptance work unless a task explicitly asks for `web/default/`.

## Stack

- React 18
- Vite
- Semi Design (`@douyinfe/semi-ui`, `@douyinfe/semi-icons`)
- i18next / react-i18next
- Axios through the existing classic helpers

## Commands

- Preview: `npm run dev`
- Build verification: `npm run build`
- Format check/fix: `npm run lint` / `npm run lint:fix`
- i18n tooling: use the existing `npm run i18n:*` scripts when changing translation coverage

## Product Rules

- Marketplace and token-management UX changes should land here first.
- If a backend contract changes, keep the classic payload shape and visible state in sync before updating compatibility UI.
- For order-pool activation, the classic marketplace page must persist the selected token's `marketplace_route_enabled` route set, and the token edit modal must show whether the order pool route is active.
