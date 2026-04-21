# HermesToken Performance Governance Design

Date: 2026-04-21
Status: Approved in chat, pending final spec review
Owner: Codex + user

## Goal

Reduce HermesToken's current white-screen startup behavior into a fast, content-first experience across both the public site and `/console`, while adding enough build-time and runtime guardrails that performance does not regress after future releases.

## Current Problems

### Live measurements taken before design

From the live site `https://hermestoken.top/`:

- First Contentful Paint is about 9.1s
- `DOMContentLoaded` and `load` both land around 9.1s
- The home page returns almost empty HTML, then waits for client-side React to render content
- The initial request count is low enough that raw network volume is not the main issue
- The biggest problem is the startup chain: HTML shell -> JS bootstrap -> i18n init -> layout init -> home-page fetches -> visible content

Measured resource highlights:

- `react-core-aowPqMBd.js` blocks for about 7.5s in browser timings
- `index-BPgerhbn.js` takes about 1.9s
- `index-htLf8YEF.css` takes about 3.5s
- HTML is tiny, but it contains only `#root` and no meaningful above-the-fold content

### Code-level causes

- `web/src/index.jsx` waits for `initializeI18n()` before calling `renderApp()`
- `web/src/components/layout/PageLayout.jsx` loads `/api/status` on all routes, including the public home page
- `web/src/pages/Home/index.jsx` separately loads `/api/home_page_content` and `/api/notice`, and dynamically imports `marked` for markdown rendering
- Public and console code paths still share too much startup cost, even though the landing page should not need console-grade runtime, data, or dependencies
- The current app behaves like a single empty-shell SPA, so any startup slowdown becomes a visible white-screen slowdown

## Constraints

- Existing product behavior must keep working for both public users and authenticated console users
- The user accepts architecture-level changes, including frontend, backend, and deployment-layer changes
- We should improve startup behavior without taking on a full SSR rewrite of the entire app
- The rollout must preserve rollback options and keep production deploy risk reasonable

## Non-Goals

- Full migration to React SSR or a new framework
- Redesigning the product's visual identity
- Rewriting every API for ideal purity in the same pass
- Solving every backend latency issue unrelated to startup-critical user experience

## Selected Approach

We will do layered performance governance instead of a narrow frontend-only patch and instead of a risky full SSR rewrite.

This design has four major moves:

1. Split public startup from console startup
2. Make the public home page content-first by serving meaningful HTML and bootstrap data immediately
3. Restructure bundles and route boundaries so console-only dependencies cannot leak into the public route
4. Add delivery, caching, compression, and performance-budget enforcement so the gains stick

## Alternative Approaches Considered

### Option A: Frontend-only tactical tuning

Examples:

- More lazy loading
- Fewer startup fetches
- Smaller chunks
- Better caching headers

Pros:

- Smallest change set
- Fastest to ship

Cons:

- Does not fix the root problem of empty HTML on the home page
- White-screen behavior improves, but does not disappear
- Public and console startup paths remain entangled

Decision: rejected as insufficient.

### Option B: Layered public/console performance governance

Examples:

- Public page gets pre-rendered or server-injected bootstrap HTML
- Console becomes a separate startup path
- Startup-critical assets get explicit budgets and chunk boundaries
- Delivery layer gets proper compression and caching

Pros:

- Fixes the actual user-visible problem
- Preserves the current React/Vite stack
- Reasonable implementation risk
- Gives us permanent regression protection

Cons:

- Touches frontend, backend, and deployment together
- Needs careful rollout and verification

Decision: selected.

### Option C: Full SSR or framework migration

Examples:

- Move public and console routes to a fully server-rendered architecture
- Replace current routing and rendering flow

Pros:

- Strong theoretical long-term control over first paint

Cons:

- Very large migration surface
- High regression risk
- Not proportional to the current problem

Decision: rejected as too invasive for this goal.

## Target Outcomes

### Public site

- The home page shows meaningful content immediately from HTML, not after React finishes booting
- Public routes do not pay for console-only startup code or data
- Home page startup no longer depends on three post-render API calls to look alive
- Markdown and notice rendering no longer sit in the critical path

### Console

- `/console` becomes its own performance domain with route-level and feature-level loading boundaries
- Heavy dependencies such as charts, markdown tooling, Mermaid, KaTeX, and large UI subsystems are loaded only when needed
- The console shell renders quickly, then feature modules stream in by route and intent

### Delivery and governance

- Versioned assets stay immutable and aggressively cacheable
- HTML stays revalidatable with ETag / no-cache semantics
- Compression and transfer strategy become explicit rather than incidental
- Build output is checked against budgets so future PRs cannot quietly bloat startup

## Architecture

### 1. Public and console startup split

The app will stop treating `/` and `/console` as one startup path.

Planned structure:

- `web/src/index.jsx` becomes a thin entry coordinator
- A new public bootstrap path is introduced, likely under `web/src/bootstrap/publicApp.jsx`
- A new console bootstrap path is introduced, likely under `web/src/bootstrap/consoleApp.jsx` or an equivalent split rooted in the router/layout layer

Responsibility split:

- Public bootstrap provides only the contexts needed for public routes
- Console bootstrap keeps the full authenticated app shell and admin/runtime concerns
- Shared code remains only where it is truly startup-safe for both sides

This makes it possible to reason about startup budgets per route family instead of per entire application.

### 2. Content-first public home page

The public home page will no longer rely on an empty HTML shell.

Instead:

- The server will return meaningful above-the-fold HTML for the home page
- A small bootstrap payload will be injected into the page, for example on `window.__HERMES_BOOTSTRAP__`
- The public React app hydrates or enhances the already-visible content instead of making the content appear from nothing

Bootstrap payload will include only public-critical fields, such as:

- system name
- logo / favicon information
- home page content or its rendered HTML form
- notice metadata or notice summary needed for first paint decisions
- language/theme defaults for startup consistency

It will not include console-only or sensitive configuration.

### 3. Home-page data-path redesign

Current home-page dependencies are:

- `/api/status`
- `/api/home_page_content`
- `/api/notice`
- dynamic import of `marked`

New design:

- Public home page reads server-injected bootstrap first
- If bootstrap exists and is valid, the page renders immediately from it
- Background refresh can later verify freshness without blocking paint
- Markdown is rendered before or during server response generation when possible, so the browser does not need to dynamically import `marked` for startup-critical content
- Notice logic becomes non-blocking and enhancement-based, not first-paint blocking

Fallback order:

1. injected bootstrap
2. local cached public payload
3. built-in static default content

This prevents a broken API response from recreating white-screen behavior.

### 4. Status API split

`/api/status` currently behaves like a global startup dependency, even for the public site.

We will split this into two conceptual data products:

- Public status: only fields required for branding, simple public navigation, and public route behavior
- Console status: the richer operational/configuration payload used after entering authenticated or console flows

The split can be implemented either as:

- separate endpoints, or
- one endpoint with public-specific bootstrap generation and console-specific later fetches

The important part is behavioral, not naming:

- public startup must not depend on the console-grade status shape
- console consumers must keep their current richer capabilities

### 5. Bundle and route governance

We will explicitly define bundle classes in `web/vite.config.js` instead of relying on incidental chunking.

Target classes:

- startup runtime
- public marketing bundle
- console runtime bundle
- markdown runtime bundle
- charts / visualization bundle
- Mermaid / diagram bundle
- KaTeX / math bundle
- large settings or editor bundles

Rules:

- Public home page must not import console runtime
- Public startup must not pull Semi-heavy runtime unless a public page actually uses it above the fold
- Markdown, Mermaid, KaTeX, and data-viz libraries are always non-startup by default
- Console feature bundles must be route-driven and intent-driven where possible

We will also add build analysis and budget checks so this structure stays enforced.

### 6. Delivery and cache strategy

Delivery policy after this redesign:

- Versioned assets: `Cache-Control: public, max-age=31536000, immutable`
- HTML: `no-cache` with validators such as `ETag`
- Public bootstrap data: short-lived or inlined into HTML depending on route
- Public API responses that are not user-specific can use short freshness windows and `stale-while-revalidate` where safe

Compression policy:

- Keep `gzip`
- Add or enable `brotli` where the production Nginx build supports it
- Prefer compressed transfer for JS, CSS, HTML, JSON, and SVG

Transport policy:

- Keep correct module preloads for truly startup-critical chunks only
- Remove preload pressure for non-critical dependencies
- CDN remains optional enhancement, not a blocker for this optimization pass

### 7. Resilience and graceful degradation

The new startup path must stay usable even when enhancement data fails.

Required behaviors:

- If injected bootstrap is missing or malformed, the public page falls back to cached data or static default content
- If notice fetch fails, no modal appears, but the home page still renders normally
- If home-page enhancement refresh fails, the page retains already rendered content
- Console route failures continue to use existing error boundaries and loading fallbacks

We are optimizing for visible usefulness before full enhancement.

## File-Level Design

### Frontend files to modify

- `web/src/index.jsx`
  - Remove the single blocking startup chain
  - Route startup to public or console bootstraps

- `web/src/App.jsx`
  - Stop treating the app as one uniform startup boundary
  - Keep route-level decisions aligned with the new public/console split

- `web/src/components/layout/PageLayout.jsx`
  - Remove global public-route dependence on `/api/status`
  - Separate public-layout data strategy from console-layout data strategy

- `web/src/pages/Home/index.jsx`
  - Read injected bootstrap and fallback caches
  - Remove startup-critical dependency on post-render API fetches
  - Move markdown and notice work off the critical path

- `web/vite.config.js`
  - Add stricter manual chunk strategy
  - Add build-analysis hooks and startup-budget checks where practical

### Frontend files likely to add

- `web/src/bootstrap/publicApp.jsx`
  - Minimal public startup root

- `web/src/bootstrap/consoleApp.jsx`
  - Full console startup root

- `web/src/helpers/bootstrapData.js` or equivalent
  - Parse, validate, and consume injected bootstrap payload safely

- `web/src/helpers/publicStartupCache.js` or equivalent
  - Cache/fallback helpers for public bootstrap content

- `web/tests/...`
  - Tests that assert public startup does not wait on network-only content
  - Tests for bootstrap parsing and fallback behavior
  - Tests that protect chunk boundaries where practical

### Backend / delivery files to modify

- `router/web-router.go`
  - Generate or attach public bootstrap data for the home page
  - Potentially return home-page-specific HTML for `/`

- `controller/misc.go`
  - Support home-page content delivery in a way that works with server bootstrap generation
  - Potentially expose a smaller public status path if endpoint splitting is chosen

- `hermestoken.top.nginx.conf`
  - Tighten compression and caching policy
  - Ensure startup-critical preload behavior is correct
  - Preserve immutable cache policy for versioned assets

- deployment scripts or config files as needed
  - Ensure any generated public bootstrap or pre-render artifacts are built and shipped correctly

## Data Flow

### Public home page flow after redesign

1. Request `/`
2. Server returns HTML with visible content and an injected public bootstrap payload
3. Browser paints meaningful content immediately
4. Public React bootstrap hydrates or enhances the page
5. Background refresh optionally checks for fresher notice/home content
6. Non-critical UI enhancements load after first paint

### Console flow after redesign

1. Request `/console/...`
2. Browser loads small console shell bundle
3. Console-specific status/config loads after shell mount
4. Route-specific heavy modules load only when that route needs them
5. Secondary tools such as charts/markdown/diagrams load on demand

## Testing Strategy

### Unit tests

- bootstrap payload parser handles missing and malformed payloads safely
- public startup cache fallback order is correct
- home-page loader prefers injected bootstrap over network fetches
- route selectors keep public and console startup separated

### Integration tests

- `/` response includes expected bootstrap markers and content
- cache headers remain correct for HTML, assets, and relevant public payloads
- public route no longer requires `/api/status` before first meaningful render
- console route still receives the data it needs

### E2E / browser tests

- home page shows visible content without waiting for post-render API fetches
- no white-screen startup regression on throttled network
- console shell appears before heavy feature bundles finish loading
- key flows such as login, home page, and representative console pages still work

### Build verification

- capture startup bundle sizes for `/`
- compare request count and startup-critical transfer size before and after
- verify that console-only bundles are not referenced by the public route's startup graph

## Performance Budgets

These are working targets for this project, not generic web-score vanity numbers.

### Public home page

- meaningful HTML content must be visible before React enhancement completes
- startup-critical requests: target <= 10
- startup-critical JS: target <= 120 KB gzip
- initial public route must not include console runtime
- FCP target: around 1.5s class on the same test path that currently lands around 9.1s
- even if 1.5s is not reached immediately, the shipped result must be materially faster than one-third of current startup delay

### Console shell

- first console paint must show shell structure quickly, even if route modules continue loading
- charts, markdown, Mermaid, KaTeX, and other heavy features are off the initial console critical path
- route entry budgets will be measured for at least one representative admin/settings page and one data-heavy page

## Rollout Plan

### Phase 1: Baseline and guardrails

- capture current live and local bundle baselines
- add tests and tooling that measure startup budgets

### Phase 2: Public startup redesign

- inject bootstrap payload into `/`
- make home page render from server-provided content first
- remove startup-critical API dependencies from the home page

### Phase 3: Console startup split

- separate public and console bootstraps
- move console-only status loading and heavy dependencies out of public startup

### Phase 4: Build and bundle governance

- tighten `vite` chunk policy
- verify startup graphs and request fan-out

### Phase 5: Delivery-layer hardening

- improve compression and cache behavior in Nginx and app delivery
- validate HTML and asset headers in production-like runs

### Phase 6: Verification and deploy

- rerun browser benchmarks
- compare before/after metrics
- ship with rollback-ready boundaries

## Risks and Mitigations

### Risk: public bootstrap leaks too much data

Mitigation:

- bootstrap payload is explicitly whitelisted
- only public-safe fields are injected
- console-only config stays in console-specific fetches

### Risk: hydration mismatch or home-page drift

Mitigation:

- use deterministic server-rendered or injected content
- keep public startup rendering logic aligned with bootstrap shape
- add browser tests for hydration and visible-content consistency

### Risk: aggressive chunking breaks route loading

Mitigation:

- route-level tests and smoke tests
- incremental bundle refactor rather than one giant bundling rewrite

### Risk: Nginx / app cache rules become inconsistent

Mitigation:

- header verification tests
- document final cache policy and validate with `curl`

## Success Criteria

This design is considered successful when all of the following are true:

- The home page displays meaningful above-the-fold content immediately from HTML
- Public startup no longer waits on `/api/status`, `/api/home_page_content`, and `/api/notice` to look alive
- Public startup JS and request count drop to a controlled, budgeted shape
- Console routes load from a separate startup path and no longer contaminate the public route
- Static asset delivery, compression, and cache rules are explicit and verified
- We can measure the gains before and after, and we have tests or budgets that keep them from regressing

## Implementation Handoff Notes

This design intentionally chooses a layered optimization strategy that fixes the current pain without forcing a full framework migration. The implementation plan should keep the rollout incremental and always preserve a working public page and a working console shell after each stage.

## Verification Checklist

- [x] `go test ./controller -run 'TestBuildPublicBootstrapPayloadReturnsPublicSubset|TestRenderPublicHomeIndexEmbedsBootstrapAndShell'`
- [x] `go test ./router -run 'TestPublicBootstrapEndpointReturnsCacheableJSON|TestInternalPublicHomeEndpointReturnsNoCacheHTML'`
- [x] `cd web && node --test tests/bootstrap-data.test.mjs tests/public-startup-entry.test.mjs tests/public-layout-mode.test.mjs tests/home-bootstrap.test.mjs tests/public-startup-budget.test.mjs`
- [x] `cd web && npm run build && npm run perf:budget`
- [x] `python3 -m unittest scripts.tests.test_prod`
- [ ] Post-deploy gate: browser benchmark on `https://hermestoken.top/` confirms meaningful HTML before React enhancement and materially lower startup timings than the 2026-04-21 baseline.
