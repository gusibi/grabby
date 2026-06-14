/**
 * intercept-hook.js — 运行在页面 MAIN world（与页面共享 window）。
 *
 * 作用：尽早（document_start）覆写 window.fetch 和 XMLHttpRequest，
 * 捕获所有"看起来是数据接口"的响应体，并通过 window.postMessage
 * 转交给 ISOLATED world 的 bridge（MAIN world 拿不到 chrome.* API）。
 *
 * 设计原则（见 docs/browser-executor-plan.md §4.3）：
 *   - hook 只做"通用捕获"，不含任何站点业务过滤逻辑；
 *   - 具体按 operation 名过滤、解析，全部放到服务端适配器做。
 *
 * 注意：本文件被注册为 document_start + world:MAIN 的 content script，
 * 会在页面任何脚本之前执行，从而能捕获首屏请求。
 */
(function () {
    'use strict';

    // 防止重复注入（同一页面可能被多次注册/导航）
    if (window.__grabbyHookInstalled) return;
    window.__grabbyHookInstalled = true;

    const MAX_BODY_LEN = 5 * 1024 * 1024; // 单条响应体上限 5MB，超出丢弃

    // 只捕获"像数据接口"的 URL，降低噪音。服务端再按 operation 精确过滤。
    function shouldCapture(url) {
        if (!url) return false;
        return (
            url.includes('/graphql') ||
            url.includes('/api/') ||
            url.includes('/i/api/')
        );
    }

    function emit(record) {
        try {
            if (record.body && record.body.length > MAX_BODY_LEN) return;
            window.postMessage(
                { __grabby: true, kind: 'capture', record: record },
                '*'
            );
        } catch (_) {
            // 序列化失败则丢弃，不影响页面
        }
    }

    // --- hook fetch ---
    const origFetch = window.fetch;
    if (origFetch) {
        window.fetch = function (...args) {
            const reqUrl = (() => {
                try {
                    const a = args[0];
                    return typeof a === 'string' ? a : (a && a.url) || '';
                } catch (_) {
                    return '';
                }
            })();
            return origFetch.apply(this, args).then((resp) => {
                try {
                    if (shouldCapture(reqUrl)) {
                        // clone 后异步读取，绝不阻塞或破坏页面对响应的消费
                        resp.clone().text().then((body) => {
                            emit({
                                url: reqUrl,
                                status: resp.status,
                                method: 'fetch',
                                body: body,
                                ts: Date.now()
                            });
                        }).catch(() => {});
                    }
                } catch (_) {}
                return resp;
            });
        };
    }

    // --- hook XMLHttpRequest ---
    const origOpen = XMLHttpRequest.prototype.open;
    const origSend = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function (method, url, ...rest) {
        this.__grabbyUrl = url;
        this.__grabbyMethod = method;
        return origOpen.call(this, method, url, ...rest);
    };
    XMLHttpRequest.prototype.send = function (...args) {
        this.addEventListener('load', function () {
            try {
                if (shouldCapture(this.__grabbyUrl)) {
                    let body = '';
                    try {
                        body = this.responseType === '' || this.responseType === 'text'
                            ? this.responseText
                            : JSON.stringify(this.response);
                    } catch (_) {
                        body = '';
                    }
                    emit({
                        url: this.__grabbyUrl,
                        status: this.status,
                        method: this.__grabbyMethod || 'xhr',
                        body: body,
                        ts: Date.now()
                    });
                }
            } catch (_) {}
        });
        return origSend.apply(this, args);
    };
})();
