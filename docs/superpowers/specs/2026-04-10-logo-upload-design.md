# Logo Upload Design

## Goal

Allow root users to upload a local Logo image from the settings page. The server stores only one current Logo file on local disk and overwrites the previous file on each successful upload.

## Requirements

- Keep the existing manual Logo URL input as a fallback.
- Add a root-only upload endpoint for a single image file.
- Store the uploaded Logo in a fixed server-local path and overwrite the previous upload.
- Expose the uploaded Logo through a stable public URL that works in both dev and embedded-frontend deployments.
- Reject invalid files with clear error messages.

## Chosen Approach

- Persist the uploaded bytes to a fixed path under `data/`.
- Serve the file through a new public `GET /api/logo` endpoint instead of relying on `web/public` or embedded frontend assets.
- After upload succeeds, update the `Logo` option to `/api/logo?v=<timestamp>` so browsers fetch the fresh image immediately.
- Keep the existing `PUT /api/option/` flow unchanged for manual URL entry.

## Validation And Security

- Root auth on upload.
- Multipart upload limited to a single file.
- Size limit on uploaded files.
- Allowlist image content types only.
- Do not preserve previous uploaded files.

## Frontend Behavior

- Add an upload control next to the existing Logo URL input.
- On success, update the form field, local storage, and `StatusContext` immediately.
- Keep the manual “设置 Logo” button for remote URLs and other manual values.
