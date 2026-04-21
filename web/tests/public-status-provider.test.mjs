import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import React, { useContext } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const providerModulePath = new URL(
  '../src/context/Status/provider.js',
  import.meta.url,
);
const publicAppPath = new URL('../src/bootstrap/publicApp.jsx', import.meta.url);

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
