import assert from "node:assert/strict";
import test from "node:test";

import { updateChannelStatus } from "../classic/src/services/channelStatus.js";

for (const { action, status } of [
  { action: "enable", status: 1 },
  { action: "disable", status: 2 },
]) {
  test(`${action} posts to the dedicated channel status endpoint and reloads state`, async () => {
    const requests = [];
    let reloads = 0;
    const api = {
      async post(url, data) {
        requests.push({ method: "post", url, data });
        return { data: { success: true, data: true } };
      },
    };

    const response = await updateChannelStatus(api, 42, status, async () => {
      reloads += 1;
    });

    assert.deepEqual(requests, [
      {
        method: "post",
        url: "/api/channel/42/status",
        data: { status },
      },
    ]);
    assert.equal(reloads, 1);
    assert.equal(response.data.success, true);
  });
}

test("rejected channel status update does not reload stale server state", async () => {
  let reloads = 0;
  const api = {
    async post() {
      return { data: { success: false, message: "rejected" } };
    },
  };

  const response = await updateChannelStatus(api, 42, 1, async () => {
    reloads += 1;
  });

  assert.equal(reloads, 0);
  assert.equal(response.data.message, "rejected");
});

test("classic preview handles the production channel status contract", async () => {
  const { createServer } = await import(
    new URL("../classic/node_modules/vite/dist/node/index.js", import.meta.url)
  );
  const classicRoot = new URL("../classic/", import.meta.url).pathname;
  const values = new Map([["channel-preview-mock", "1"]]);
  const originalWindow = globalThis.window;
  const originalLocalStorage = globalThis.localStorage;
  globalThis.window = {
    location: { origin: "http://localhost", search: "" },
  };
  globalThis.localStorage = {
    getItem(key) {
      return values.get(key) ?? null;
    },
    removeItem(key) {
      values.delete(key);
    },
    setItem(key, value) {
      values.set(key, String(value));
    },
  };

  const server = await createServer({
    appType: "custom",
    configFile: false,
    logLevel: "silent",
    root: classicRoot,
    server: { middlewareMode: true },
  });

  try {
    const preview = await server.ssrLoadModule(
      "/src/helpers/channelPreviewMock.js",
    );
    const legacyUpdate = preview.getChannelPreviewApiResponse(
      "put",
      "/api/channel/",
      { id: 101, status: 2 },
    );
    assert.equal(legacyUpdate.data.success, false);

    const before = preview.getChannelPreviewApiResponse(
      "get",
      "/api/channel/?p=1&page_size=100",
    );
    const beforeChannel = before.data.data.items.find(({ id }) => id === 101);
    assert.equal(beforeChannel.status, 1);

    const update = preview.getChannelPreviewApiResponse(
      "post",
      "/api/channel/101/status",
      { status: 2 },
    );

    assert.equal(update?.data?.success, true);
    assert.equal(update?.data?.data, true);

    const unchanged = preview.getChannelPreviewApiResponse(
      "post",
      "/api/channel/101/status",
      { status: 2 },
    );
    assert.equal(unchanged.data.success, true);
    assert.equal(unchanged.data.data, false);

    const invalid = preview.getChannelPreviewApiResponse(
      "post",
      "/api/channel/101/status",
      { status: 3 },
    );
    assert.equal(invalid.data.success, false);

    const missing = preview.getChannelPreviewApiResponse(
      "post",
      "/api/channel/999999/status",
      { status: 1 },
    );
    assert.equal(missing.data.success, true);
    assert.equal(missing.data.data, false);

    const malformed = preview.getChannelPreviewApiResponse(
      "post",
      "/api/channel/not-a-number/status",
      { status: 1 },
    );
    assert.equal(malformed.data.success, false);

    const list = preview.getChannelPreviewApiResponse(
      "get",
      "/api/channel/?p=1&page_size=100",
    );
    const channel = list.data.data.items.find(({ id }) => id === 101);
    assert.equal(channel.status, 2);
  } finally {
    await server.close();
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
    if (originalLocalStorage === undefined) {
      delete globalThis.localStorage;
    } else {
      globalThis.localStorage = originalLocalStorage;
    }
  }
});
