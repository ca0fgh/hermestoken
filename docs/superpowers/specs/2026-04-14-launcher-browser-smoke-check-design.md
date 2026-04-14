# Launcher Browser Smoke Check Design

Date: 2026-04-14
Owner: Codex
Status: Draft for review

## Context

The current local/public launcher flow treats `HTTP 200` as the success condition for a rebuild:

- `scripts/local.py` runs `docker compose up -d --build`
- it then waits for `local_url` to return a healthy HTTP response
- `scripts/public.py` reuses that local startup flow, starts `cloudflared`, and then waits for `public_url` to return a healthy HTTP response

This misses a real failure mode that happened in production-like use:

- the backend serves `200 OK`
- the browser downloads HTML and assets successfully
- the frontend crashes during startup because of runtime JavaScript errors
- the page shows a white screen
- the launcher still prints success

The launcher should treat “white screen after rebuild” as a failed startup, not a healthy one.

## Goals

1. Make `scripts/local.py` fail when the rebuilt homepage returns `200` but does not render.
2. Make `scripts/public.py` fail when the public homepage returns `200` but does not render.
3. Surface actionable browser-side diagnostics so rebuild failures are understandable.
4. Keep the existing Docker/tunnel orchestration and HTTP health checks intact.

## Non-Goals

1. Do not add authenticated flow testing.
2. Do not add full Playwright-style interaction coverage to launcher scripts.
3. Do not change the Docker build strategy in this task.
4. Do not validate every route in the app; only the homepage smoke check is in scope.

## Proposed Approach

Add a launcher-level browser smoke check helper that runs after HTTP health passes.

The smoke check will:

1. Locate an available local browser executable.
   - Prefer Google Chrome on macOS if present.
   - Fall back to Chromium-compatible executables if available.
2. Launch the browser in headless mode with an isolated temporary profile.
3. Open the target URL.
4. Inspect runtime state through a lightweight DevTools Protocol session.
5. Fail if any startup JavaScript exception occurs or if the rendered app root is empty.

This keeps the launcher logic simple while directly testing the failure mode we care about: “can a real browser render the homepage after rebuild?”

## Success Criteria

A browser smoke check is considered successful only when all of the following are true:

1. The page title can be read successfully.
2. No `Runtime.exceptionThrown` event is observed during startup.
3. The `#root` element exists and contains non-empty rendered HTML.
4. `document.body.innerText` is non-empty after load settles.

A smoke check failure should raise a `LauncherError` with the first useful diagnostic message available, such as:

- the first runtime exception description
- a page-load timeout
- browser launch failure
- a missing browser executable warning when strict execution is enabled

## Integration Plan

### `scripts/launcher_common.py`

Add a reusable helper, tentatively named `run_browser_smoke_check(...)`, responsible for:

1. browser discovery
2. temporary profile management
3. headless launch
4. DevTools Protocol interaction
5. collecting result diagnostics
6. cleaning up the browser process and temporary directory

Add a small result shape internally so callers can distinguish:

- passed
- skipped because no browser was found
- failed with diagnostic details

The public API exposed to launcher callers should still be simple and raise `LauncherError` on hard failure.

### `scripts/local.py`

Keep the current order:

1. `docker compose up -d --build`
2. HTTP health check for `local_url`
3. browser smoke check for `local_url`

If the browser smoke check fails:

- print recent container status just like the current HTTP health failure path
- then exit non-zero with an actionable error

### `scripts/public.py`

Keep the current order:

1. run local launcher flow
2. start tunnel
3. HTTP health check for `public_url`
4. browser smoke check for `public_url`

If the public smoke check fails:

- stop the new tunnel process started by this run
- include recent cloudflared log context when available
- exit non-zero with an actionable error

## Browser Discovery Rules

The launcher should try a short ordered list of local browser binaries.

Initial target list:

1. `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`
2. `/Applications/Chromium.app/Contents/MacOS/Chromium`
3. `google-chrome`
4. `chromium`
5. `chromium-browser`
6. `chrome`

If none are found:

- print a visible warning that browser smoke checking was skipped
- continue the launcher flow successfully for now

This preserves usability on machines without a local browser while still improving validation on typical developer machines.

## Failure Messaging

Failure messages should be explicit and short.

Examples:

- `Browser smoke check failed for http://localhost:3000: TypeError: Cannot read properties of undefined (reading 'Component') in /assets/semi-ui-....js`
- `Browser smoke check failed for https://pay-local.hermestoken.top: root element remained empty after page load.`
- `Browser smoke check failed for http://localhost:3000: browser could not start. Next step: Verify Chrome/Chromium is installed and launchable.`

Skip messaging should look like:

- `[warn] Browser smoke check skipped for http://localhost:3000: Chrome/Chromium not found`

## Test Plan

### Unit/launcher tests

Update existing Python tests to cover:

1. `local.run_local_stack(...)` invokes browser smoke check after HTTP health succeeds.
2. `public.run_public_stack(...)` invokes browser smoke check after public HTTP health succeeds.
3. browser smoke check failures propagate as actionable launcher failures.
4. “no browser found” produces a skip warning without failing startup.
5. `public.py` still stops the tunnel process on smoke-check failure.

### Manual verification

After implementation:

1. Run launcher test suite:
   - `python3 -m unittest scripts.tests.test_dockerfile_frontend_build scripts.tests.test_local scripts.tests.test_public`
2. Rebuild local stack:
   - `python3 scripts/local.py`
3. Rebuild public stack:
   - `python3 scripts/public.py`
4. Verify both URLs render in a clean browser:
   - `http://localhost:3000/`
   - `https://pay-local.hermestoken.top/`

## Risks and Mitigations

### Risk: false negatives from brittle timing

Mitigation:

- wait for load to settle for a bounded amount of time
- treat the smoke check as “startup render succeeded” instead of asserting specific UI copy beyond non-empty root/body content

### Risk: browser not installed on every machine

Mitigation:

- support skip-with-warning behavior when no browser binary is found
- keep existing HTTP health checks as baseline validation

### Risk: public smoke check leaves background browser processes behind

Mitigation:

- always track child PID/process handle
- always clean up browser process and temporary profile directory in `finally`

## Recommended Implementation Boundary

Keep this task limited to launcher scripts and tests:

- `scripts/launcher_common.py`
- `scripts/local.py`
- `scripts/public.py`
- `scripts/tests/test_local.py`
- `scripts/tests/test_public.py`

No app runtime code changes are required for this task.
