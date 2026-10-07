const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
function harness(local = {}) {
    const timers = new Map();
    let id = 0;
    const sockets = [];
    const context = vm.createContext({
        console: { log() {}, warn() {}, error() {} }, URL, AbortController,
        setTimeout(fn, ms) { timers.set(++id, { fn, ms }); return id; },
        clearTimeout(id) { timers.delete(id); }, setInterval() {}, clearInterval() {},
        fetch: async () => ({ ok: true }),
        chrome: {
            storage: {
                sync: { get(keys, cb) { cb({ serverUrl: 'ws://localhost:8080/ws', browserName: 'test' }); } },
                local: { get(keys, cb) { cb({ browserConnectId: 'test-id', ...local }); }, set(data) { Object.assign(local, data); return Promise.resolve(); } },
                onChanged: { addListener() {} }
            }, runtime: { sendMessage: async () => {} }
        },
        WebSocket: class {
            static OPEN = 1; static CONNECTING = 0;
            constructor() { this.readyState = 0; sockets.push(this); }
            close() { this.readyState = 3; }
        }, self: {}
    });
    vm.runInContext(fs.readFileSync(require.resolve('../lib/websocket.js'), 'utf8'), context);
    return { manager: context.self.websocketManager, timers, sockets, context, local };
}
test('server restart retries every ten seconds until connected', async () => {
    const { manager, timers, sockets } = harness();
    await manager.connect();
    sockets[0].readyState = 1; sockets[0].onopen({});
    sockets[0].onclose({ target: sockets[0], code: 1006, wasClean: false });
    const retry = [...timers.values()].find(t => t.ms === 10000);
    assert.ok(retry, 'missing ten-second retry');
    await retry.fn();
    assert.equal(sockets.length, 2);
});
test('manual disconnect survives worker recreation', async () => {
    const first = harness();
    await first.manager.connect();
    first.manager.disconnect();
    const second = harness(first.local);
    if (second.manager.reconnectStateReady) await second.manager.reconnectStateReady;
    assert.equal(second.manager.autoReconnect, false);
});
test('manual disconnect cancels registration in flight', async () => {
    const { manager, sockets, context } = harness();
    let finish;
    context.fetch = () => new Promise(resolve => { finish = resolve; });
    const connecting = manager.connect();
    while (!finish) await Promise.resolve();
    manager.disconnect();
    finish({ ok: true });
    await connecting;
    assert.equal(sockets.length, 0);
});

test('manual disconnect stops retries and manual connect enables them again', async () => {
    const { manager, timers, sockets, local } = harness();
    await manager.connect();
    sockets[0].onclose({ target: sockets[0], code: 1006, wasClean: false });
    manager.disconnect();
    assert.equal(timers.size, 0);
    await manager.connect(false);
    assert.equal(sockets.length, 1);
    await manager.connect();
    assert.equal(sockets.length, 2);
    assert.equal(local.autoReconnectEnabled, true);
});
test('registration failure keeps retrying while server is down', async () => {
    const { manager, timers, context } = harness();
    context.fetch = async () => { throw new Error('server offline'); };
    await manager.connect();
    for (let i = 0; i < 3; i++) {
        const entry = [...timers.entries()].find(([, t]) => t.ms === 10000);
        assert.ok(entry);
        timers.delete(entry[0]);
        await entry[1].fn();
    }
    assert.equal(manager.autoReconnect, true);
});
