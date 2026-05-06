import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const tokenVerificationPagePath = new URL(
  "../classic/src/pages/TokenVerification/index.jsx",
  import.meta.url,
);
const classicLocaleDir = new URL(
  "../classic/src/i18n/locales/",
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

test("token verification copy does not expose internal probe engine branding", async () => {
  const source = await readFile(tokenVerificationPagePath, "utf8");
  assert.doesNotMatch(source, /LLMprobe-engine/);

  const localeFiles = await readdir(classicLocaleDir);
  await Promise.all(
    localeFiles
      .filter((file) => file.endsWith(".json"))
      .map(async (file) => {
        const localeSource = await readFile(
          new URL(file, classicLocaleDir),
          "utf8",
        );
        assert.doesNotMatch(
          localeSource,
          /LLMprobe-engine/,
          `${file} should not expose internal probe engine branding`,
        );
      }),
  );
});

test("token verification supports Anthropic Claude Code client profile", async () => {
  const source = await readFile(tokenVerificationPagePath, "utf8");

  assert.match(source, /value:\s*'claude_code'/);
  assert.match(source, /客户端模式/);
  assert.match(source, /provider\s*===\s*'anthropic'/);
  assert.match(
    source,
    /client_profile:\s*provider\s*===\s*'anthropic'\s*\?\s*clientProfile\s*:\s*''/,
  );
  assert.match(source, /probeResult\?\.client_profile\s*\|\|\s*clientProfile/);
});

test("token verification renders structured probe risk status and evidence", async () => {
  const source = await readFile(tokenVerificationPagePath, "utf8");

  assert.match(source, /function probeStatusMeta\(record,\s*t\)/);
  assert.match(source, /record\.error_code\s*===\s*'judge_unconfigured'/);
  assert.match(source, /label:\s*t\('未评分'\)/);
  assert.match(source, /riskLevel\s*===\s*'high'/);
  assert.match(source, /label:\s*t\('高危'\)/);
  assert.match(source, /riskLevel\s*===\s*'medium'/);
  assert.match(source, /label:\s*t\('中危'\)/);
  assert.match(source, /label:\s*t\('低风险'\)/);
  assert.match(source, /Array\.isArray\(record\.evidence\)/);
  assert.match(source, /evidence\.slice\(0,\s*3\)\.map/);
});

test("token verification can export direct probe evidence for corpus review", async () => {
  const source = await readFile(tokenVerificationPagePath, "utf8");

  assert.match(source, /downloadTextAsFile/);
  assert.match(source, /function buildProbeEvidenceFilename\(result\)/);
  assert.match(source, /function exportProbeEvidence\(\)/);
  assert.match(source, /JSON\.stringify\(probeResult,\s*null,\s*2\)/);
  assert.match(source, /token-verification-evidence/);
  assert.match(source, /导出证据/);
  assert.match(source, /disabled=\{!probeResult\s*\|\|\s*probing\}/);
});

test("token verification makes weak identity evidence explicit", async () => {
  const source = await readFile(tokenVerificationPagePath, "utf8");

  assert.match(source, /uncertain:\s*'证据不足'/);
  assert.match(source, /function renderIdentityPredictedFamily\(value,\s*record,\s*t\)/);
  assert.match(
    source,
    /record\.status\s*===\s*'uncertain'[\s\S]*return\s+t\('证据不足'\)/,
  );
  assert.match(
    source,
    /function renderIdentityEvidence\(items\s*=\s*\[\],\s*record\s*=\s*\{\},\s*t/,
  );
  assert.match(source, /record\.verdict\?\.status\s*===\s*'insufficient_data'/);
  assert.match(
    source,
    /t\('当前信号不足，建议重跑 Deep\/Full 探针或查看原始响应'\)/,
  );
  assert.match(source, /不能作为身份不一致结论/);
});

test("token verification locale files define structured risk copy", async () => {
  const requiredKeys = [
    "未评分",
    "高危",
    "中危",
    "低风险",
    "证据不足",
    "客户端模式",
    "当前信号不足，建议重跑 Deep/Full 探针或查看原始响应",
    "交叉判定数据不足，不能作为身份不一致结论",
  ];
  const localeFiles = await readdir(classicLocaleDir);
  await Promise.all(
    localeFiles
      .filter((file) => file.endsWith(".json"))
      .map(async (file) => {
        const localeSource = await readFile(
          new URL(file, classicLocaleDir),
          "utf8",
        );
        const translation = JSON.parse(localeSource).translation;
        for (const key of requiredKeys) {
          assert.ok(
            translation?.[key],
            `${file} should define non-empty translation for ${key}`,
          );
        }
      }),
  );
});
