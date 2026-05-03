import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  getFooterSectionVisibility,
  normalizeHeaderNavModules,
} from "../classic/src/helpers/headerNavModules.js";

const footerPath = new URL(
  "../classic/src/components/layout/Footer.jsx",
  import.meta.url,
);

test("normalizeHeaderNavModules falls back to defaults for empty config", () => {
  assert.deepEqual(normalizeHeaderNavModules(""), {
    home: true,
    console: true,
    pricing: {
      enabled: true,
      requireAuth: false,
    },
    marketplace: true,
    verification: true,
    docs: true,
    about: true,
  });
});

test("normalizeHeaderNavModules upgrades legacy pricing boolean config", () => {
  assert.deepEqual(
    normalizeHeaderNavModules(
      JSON.stringify({
        home: true,
        console: true,
        pricing: false,
        docs: false,
        about: true,
      }),
    ),
    {
      home: true,
      console: true,
      pricing: {
        enabled: false,
        requireAuth: false,
      },
      marketplace: true,
      verification: true,
      docs: false,
      about: true,
    },
  );
});

test("normalizeHeaderNavModules enables marketplace for legacy saved config", () => {
  assert.deepEqual(
    normalizeHeaderNavModules(
      JSON.stringify({
        home: true,
        console: true,
        pricing: {
          enabled: true,
          requireAuth: false,
        },
        docs: true,
        about: true,
      }),
    ),
    {
      home: true,
      console: true,
      pricing: {
        enabled: true,
        requireAuth: false,
      },
      marketplace: true,
      verification: true,
      docs: true,
      about: true,
    },
  );
});

test("classic navigation places token verification immediately after marketplace", async () => {
  const source = await readFile(
    new URL("../classic/src/hooks/common/useNavigation.js", import.meta.url),
    "utf8",
  );

  assert.match(
    source,
    /itemKey:\s*'marketplace'[\s\S]*?itemKey:\s*'verification'[\s\S]*?itemKey:\s*'console'/,
  );
  assert.match(source, /text:\s*t\('检测'\)/);
  assert.match(source, /to:\s*'\/token-verification'/);
});

test("classic token verification route is gated by the header module and login", async () => {
  const [appSource, routeSource, pageSource] = await Promise.all([
    readFile(new URL("../classic/src/App.jsx", import.meta.url), "utf8"),
    readFile(
      new URL("../classic/src/routes/PublicRoutes.jsx", import.meta.url),
      "utf8",
    ),
    readFile(
      new URL(
        "../classic/src/pages/TokenVerification/index.jsx",
        import.meta.url,
      ),
      "utf8",
    ),
  ]);

  assert.match(appSource, /verificationEnabled/);
  assert.match(appSource, /if \(!statusState\?\.status\) \{\s*return true;/);
  assert.match(routeSource, /path='\/token-verification'/);
  assert.match(
    routeSource,
    /verificationEnabled\s*\?\s*\(\s*<PrivateRoute>[\s\S]*?<TokenVerification/,
  );
  assert.match(pageSource, /\/api\/token_verification\/probe/);
  assert.match(pageSource, /base_url/);
  assert.match(pageSource, /api_key/);
  assert.match(pageSource, /model/);
  assert.match(pageSource, /gpt-5\.5/);
  assert.match(pageSource, /gpt-5\.4/);
  assert.match(pageSource, /claude-opus-4-7/);
  assert.match(pageSource, /claude-opus-4-6/);
  assert.match(pageSource, /allowCreate/);
  assert.doesNotMatch(pageSource, /token-verification-model-presets/);
  assert.doesNotMatch(pageSource, /\/api\/token\//);
});

test("getFooterSectionVisibility hides docs and about sections with the same switches as the header", () => {
  const modules = normalizeHeaderNavModules(
    JSON.stringify({
      home: true,
      console: true,
      pricing: {
        enabled: true,
        requireAuth: false,
      },
      docs: false,
      about: false,
    }),
  );

  assert.deepEqual(
    getFooterSectionVisibility(modules, "https://docs.example.com"),
    {
      showDocsSection: false,
      showAboutSection: false,
    },
  );
});

test("footer uses shared header-nav visibility instead of hard-coded docs/about sections", async () => {
  const source = await readFile(footerPath, "utf8");

  assert.match(source, /getFooterSectionVisibility/);
  assert.match(source, /showDocsSection/);
  assert.match(source, /showAboutSection/);
});

test("settings header nav source separates marketplace entry visibility from guest access copy", async () => {
  const source = await readFile(
    new URL(
      "../classic/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx",
      import.meta.url,
    ),
    "utf8",
  );

  assert.match(source, /t\('控制是否显示模型广场入口'\)/);
  assert.match(
    source,
    /t\(\s*'关闭后游客按 default 分组浏览模型广场，登录用户可查看全部展示模型'/,
  );
  assert.match(source, /key:\s*'marketplace'/);
  assert.match(source, /t\('控制是否显示市场入口'\)/);
  assert.match(source, /key:\s*'verification'/);
  assert.match(source, /t\('控制是否显示 Token 质量黑盒检测入口'\)/);
});

test("settings header nav cards use flex grid layout to avoid float masonry gaps", async () => {
  const source = await readFile(
    new URL(
      "../classic/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx",
      import.meta.url,
    ),
    "utf8",
  );

  assert.match(source, /<Row[^>]*\btype='flex'/);
  assert.match(source, /<Row[^>]*\balign='top'/);
});
