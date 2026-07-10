import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import test from 'node:test';

// This directory is not a workspace, so a bare `react` import here resolves to
// the hoisted React 19 that web/default uses, while the classic component under
// test resolves classic's React 18. Rendering across those two copies leaves the
// hook dispatcher null. Anchor the renderer to classic's React instead.
const requireFromClassic = createRequire(new URL('../classic/package.json', import.meta.url));
const React = requireFromClassic('react');
const { useContext } = React;
const { renderToStaticMarkup } = requireFromClassic('react-dom/server');

const providerModulePath = new URL(
  '../classic/src/context/Status/provider.js',
  import.meta.url,
);
const publicAppPath = new URL('../classic/src/bootstrap/publicApp.jsx', import.meta.url);

test('StatusProvider exposes seeded initial status through StatusContext on first render', async () => {
  const { StatusContext, StatusProvider } = await import(providerModulePath);

  function StatusProbe() {
    const [statusState] = useContext(StatusContext);
    return React.createElement(
      'output',
      null,
      statusState?.status?.system_name || 'missing',
    );
  }

  const markup = renderToStaticMarkup(
    React.createElement(
      StatusProvider,
      {
        initialStatus: {
          system_name: 'HermesToken Bootstrap',
        },
      },
      React.createElement(StatusProbe),
    ),
  );

  assert.match(markup, /HermesToken Bootstrap/);
});

test('public bootstrap wrapper seeds StatusProvider with injected bootstrap status', async () => {
  const source = await readFile(publicAppPath, 'utf8');

  assert.match(source, /<StatusProvider initialStatus=\{injectedBootstrap\?\.status\}>/);
});
