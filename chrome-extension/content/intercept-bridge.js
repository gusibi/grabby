/**
 * intercept-bridge.js — 运行在 ISOLATED world（扩展隔离环境，可用 chrome.* API）。
 *
 * 作用：作为 MAIN world hook 与扩展 background 之间的桥。
 *   - 监听 window 'message'，把 hook 捕获到的响应缓冲在本 world；
 *   - 响应 background 的 'grabby:flush' 请求，回传并清空缓冲。
 *
 * 为什么需要它：MAIN world 与页面共享环境、能覆写 fetch，但拿不到
 * chrome.runtime；content script 默认隔离、能用 chrome.* 但看不到页面
 * 覆写的 fetch。两者经 window.postMessage 衔接。见 plan §4.3(a)。
 *
 * 注册为 document_start + world:ISOLATED，与 hook 同时尽早就位。
 */
(function () {
    'use strict';

    if (window.__grabbyBridgeInstalled) return;
    window.__grabbyBridgeInstalled = true;

    const MAX_BUFFER = 500; // 最多缓冲 500 条，超出丢弃最旧的
    const buffer = [];

    window.addEventListener('message', (event) => {
        // 只接受同源、本扩展约定格式的消息
        if (event.source !== window) return;
        const data = event.data;
        if (!data || data.__grabby !== true || data.kind !== 'capture') return;
        buffer.push(data.record);
        if (buffer.length > MAX_BUFFER) buffer.shift();
    });

    chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
        if (!msg || msg.type !== 'grabby:flush') return false;
        // 返回当前缓冲快照；drain=true 时清空
        const snapshot = buffer.slice();
        if (msg.drain) buffer.length = 0;
        sendResponse({ ok: true, captures: snapshot });
        return true; // 同步已响应
    });
})();
