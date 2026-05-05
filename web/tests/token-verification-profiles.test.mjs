import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const tokenVerificationPagePath = new URL(
  "../classic/src/pages/TokenVerification/index.jsx",
  import.meta.url,
);

test("token verification profile selector exposes standard and deep to users, full to admins", async () => {
  const source = await readFile(tokenVerificationPagePath, "utf8");

  assert.match(source, /value:\s*'standard'/);
  assert.match(source, /value:\s*'deep'/);
  assert.match(source, /value:\s*'full'/);
  assert.match(source, /深度检测/);
  assert.match(
    source,
    /if\s*\(\s*isAdminUser\s*\)\s*{\s*options\.push\(\{\s*label:\s*t\('完整检测'\),\s*value:\s*'full'\s*\}\)/s,
  );
  assert.doesNotMatch(
    source,
    /probe_profile:\s*isAdminUser\s*\?\s*probeProfile\s*:\s*'standard'/,
  );
  assert.match(
    source,
    /if\s*\(\s*!isAdminUser\s*&&\s*probeProfile\s*===\s*'full'\s*\)\s*{\s*setProbeProfile\('deep'\)/s,
  );
});
