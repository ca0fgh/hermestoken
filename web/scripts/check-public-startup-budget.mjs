import { gzipSync } from 'node:zlib';
import { readFile } from 'node:fs/promises';
import path from 'node:path';

const PUBLIC_STARTUP_REQUEST_BUDGET = 10;
const PUBLIC_STARTUP_JS_GZIP_BUDGET = 120 * 1024;
const PUBLIC_STARTUP_TOTAL_GZIP_BUDGET = 150 * 1024;
const JAVASCRIPT_ASSET_PATTERN = /\.(?:[cm]?js)$/;
const STYLESHEET_LINK_PATTERN =
  /<link\s+[^>]*rel="stylesheet"[^>]*href="([^"]+)"[^>]*>/gi;
const MODULE_PRELOAD_LINK_PATTERN =
  /<link\s+[^>]*rel="modulepreload"[^>]*href="([^"]+)"[^>]*>/gi;

export function evaluatePublicStartupBudget({
  requestCount,
  totalStartupGzipBytes,
  jsStartupGzipBytes,
}) {
  const errors = [];

  if (requestCount > PUBLIC_STARTUP_REQUEST_BUDGET) {
    errors.push(
      `public startup request budget exceeded: ${requestCount} requests > ${PUBLIC_STARTUP_REQUEST_BUDGET} request budget`,
    );
  }

  if (jsStartupGzipBytes > PUBLIC_STARTUP_JS_GZIP_BUDGET) {
    errors.push(
      `public startup JavaScript budget exceeded: ${formatKiB(
        jsStartupGzipBytes,
      )} > ${formatKiB(PUBLIC_STARTUP_JS_GZIP_BUDGET)}`,
    );
  }

  if (totalStartupGzipBytes > PUBLIC_STARTUP_TOTAL_GZIP_BUDGET) {
    errors.push(
      `public startup total startup budget exceeded: ${formatKiB(
        totalStartupGzipBytes,
      )} > ${formatKiB(PUBLIC_STARTUP_TOTAL_GZIP_BUDGET)}`,
    );
  }

  return { errors };
}

function formatKiB(bytes) {
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

function isJavaScriptAsset(fileName) {
  return JAVASCRIPT_ASSET_PATTERN.test(fileName);
}

async function gzipAssetFile(filePath) {
  const fileBuffer = await readFile(filePath);
  return gzipSync(fileBuffer).byteLength;
}

async function readManifest(distDir) {
  const manifestPath = path.join(distDir, '.vite', 'manifest.json');
  return JSON.parse(await readFile(manifestPath, 'utf8'));
}

async function readBuiltIndexHtml(distDir) {
  const indexHtmlPath = path.join(distDir, 'index.html');
  return readFile(indexHtmlPath, 'utf8');
}

function normalizeBuiltAssetPath(assetPath) {
  const normalizedSource = String(assetPath || '')
    .trim()
    .replace(/^\.\//, '');
  const pathname = (() => {
    try {
      return new URL(normalizedSource).pathname;
    } catch {
      return normalizedSource;
    }
  })()
    .split(/[?#]/, 1)[0]
    .replace(/^\/+/, '');

  const assetMatch = pathname.match(/(?:^|\/)(assets\/.+)$/);
  if (assetMatch) {
    return assetMatch[1];
  }

  const viteMatch = pathname.match(/(?:^|\/)(\.vite\/.+)$/);
  if (viteMatch) {
    return viteMatch[1];
  }

  return pathname;
}

function collectHtmlAssetReferences(indexHtml, pattern) {
  const references = [];
  let match;

  while ((match = pattern.exec(indexHtml)) !== null) {
    references.push(normalizeBuiltAssetPath(match[1]));
  }

  return references;
}

function findPublicEntryFile(indexHtml) {
  const entryMatch = indexHtml.match(
    /<script\s+type="module"[^>]*src="([^"]+)"[^>]*><\/script>/i,
  );

  if (!entryMatch) {
    throw new Error('Could not find the public startup entry script in dist/index.html.');
  }

  return normalizeBuiltAssetPath(entryMatch[1]);
}

function findManifestEntryByFile(manifest, entryFile) {
  const normalizedEntryFile = normalizeBuiltAssetPath(entryFile);

  for (const [manifestKey, manifestEntry] of Object.entries(manifest)) {
    if (normalizeBuiltAssetPath(manifestEntry?.file) === normalizedEntryFile) {
      return { manifestKey, manifestEntry };
    }
  }

  throw new Error(
    `Could not map built entry asset "${entryFile}" back to the Vite manifest.`,
  );
}

export function resolvePublicStartupRequestFiles({ indexHtml, manifest }) {
  const entryFile = findPublicEntryFile(indexHtml);
  const stylesheetFiles = new Set(
    collectHtmlAssetReferences(indexHtml, STYLESHEET_LINK_PATTERN),
  );
  const modulePreloadFiles = collectHtmlAssetReferences(
    indexHtml,
    MODULE_PRELOAD_LINK_PATTERN,
  );
  const requestFiles = new Set([
    entryFile,
    ...modulePreloadFiles,
    ...stylesheetFiles,
  ]);
  const visitedKeys = new Set();
  const pendingKeys = [];
  const entryManifestMatch = findManifestEntryByFile(manifest, entryFile);

  for (const rootFile of [entryFile, ...modulePreloadFiles]) {
    const manifestMatch = findManifestEntryByFile(manifest, rootFile);
    if (manifestMatch.manifestKey !== 'index.html') {
      pendingKeys.push(manifestMatch.manifestKey);
    }
  }

  for (const importedManifestKey of entryManifestMatch.manifestEntry.imports || []) {
    pendingKeys.push(importedManifestKey);
  }

  while (pendingKeys.length > 0) {
    const manifestKey = pendingKeys.pop();
    if (!manifestKey || visitedKeys.has(manifestKey)) {
      continue;
    }

    visitedKeys.add(manifestKey);

    const manifestEntry = manifest[manifestKey];
    if (!manifestEntry) {
      continue;
    }

    if (manifestEntry.file) {
      requestFiles.add(manifestEntry.file);
    }

    for (const cssFile of manifestEntry.css || []) {
      if (stylesheetFiles.has(cssFile)) {
        requestFiles.add(cssFile);
      }
    }

    for (const importedManifestKey of manifestEntry.imports || []) {
      pendingKeys.push(importedManifestKey);
    }
  }

  return [...requestFiles].sort();
}

function collectStartupAssets(indexHtml, manifest) {
  const requestFiles = resolvePublicStartupRequestFiles({
    indexHtml,
    manifest,
  });
  const jsFiles = requestFiles.filter((fileName) => isJavaScriptAsset(fileName));

  return {
    requestFiles,
    jsFiles,
  };
}

async function measureStartupAssets(distDir, requestFiles, jsFiles) {
  const jsFileSet = new Set(jsFiles);
  const requestGzipSizes = await Promise.all(
    requestFiles.map(async (fileName) => {
      const gzipBytes = await gzipAssetFile(path.join(distDir, fileName));
      return [fileName, gzipBytes];
    }),
  );
  const jsStartupGzipBytes = requestGzipSizes.reduce((total, [fileName, gzipBytes]) => {
    return total + (jsFileSet.has(fileName) ? gzipBytes : 0);
  }, 0);
  const totalStartupGzipBytes = requestGzipSizes.reduce((total, [, gzipBytes]) => {
    return total + gzipBytes;
  }, 0);

  return {
    jsStartupGzipBytes,
    totalStartupGzipBytes,
  };
}

async function collectPublicStartupStats({ cwd = process.cwd() } = {}) {
  const distDir = path.join(cwd, 'dist');
  const [manifest, indexHtml] = await Promise.all([
    readManifest(distDir),
    readBuiltIndexHtml(distDir),
  ]);
  const entryFile = findPublicEntryFile(indexHtml);
  const { manifestKey, manifestEntry } = findManifestEntryByFile(manifest, entryFile);
  const { requestFiles, jsFiles } = collectStartupAssets(indexHtml, manifest);
  const { jsStartupGzipBytes, totalStartupGzipBytes } = await measureStartupAssets(
    distDir,
    requestFiles,
    jsFiles,
  );

  return {
    entryFile,
    manifestEntry: manifestKey,
    requestCount: requestFiles.length,
    requestFiles,
    jsStartupGzipBytes,
    totalStartupGzipBytes,
    entryCssFiles: manifestEntry.css || [],
  };
}

async function run() {
  const stats = await collectPublicStartupStats();
  const { errors } = evaluatePublicStartupBudget(stats);

  if (errors.length > 0) {
    throw new Error(errors.join('\n'));
  }

  console.log(JSON.stringify(stats, null, 2));
}

if (import.meta.url === new URL(process.argv[1], 'file:').href) {
  run().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
