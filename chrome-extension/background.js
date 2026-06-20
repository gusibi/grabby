/**
 * 后台脚本
 * 负责在浏览器后台运行，管理WebSocket连接和处理消息
 */

// 导入模块
importScripts('./lib/websocket.js', './lib/capture.js', './lib/extractor.js', './lib/logger.js');

// 初始化日志记录器
const logger = new Logger();

// 初始化模块实例
const websocketManager = new WebSocketManager();
const captureManager = new CaptureManager();
const contentExtractor = new ContentExtractor();

// 连接状态
let connectionStatus = {
    status: 'disconnected', // 添加status属性，初始值为'disconnected'
    connected: false,
    serverUrl: null,
    lastConnected: null,
    lastError: null
};

// 初始化后台脚本
function init() {
    // 设置初始图标为未连接状态（灰色）
    updateToolbarIcon('disconnected');

    // 监听来自弹出窗口和选项页面的消息
    chrome.runtime.onMessage.addListener(handleMessage);


    // 监听WebSocket连接状态变化
    websocketManager.on('status_change', handleConnectionStatusChange);


    // 监听WebSocket消息
    logger.log('background', 'info', '注册WebSocket消息监听器');
    websocketManager.on('message', handleWebSocketMessage);


    // 加载设置并自动连接（如果启用）
    loadSettingsAndConnect();

}

// 加载设置并自动连接
function loadSettingsAndConnect() {
    chrome.storage.sync.get(['serverUrl', 'autoConnect'], (result) => {
        // 确保WebSocketManager实例有最新的serverUrl
        if (result.serverUrl) {
            websocketManager.serverUrl = result.serverUrl;

        }

        if (result.autoConnect && result.serverUrl) {
            // 自动连接
            connectToServer();
        }
    });
}

// 处理消息
function handleMessage(message, sender, sendResponse) {

    switch (message.action) {
        case 'connect':
            connectToServer()
                .then(status => sendResponse(status))
                .catch(error => sendResponse({ success: false, error: error.message }));
            return true; // 异步响应
        case 'disconnect':
            disconnectFromServer()
                .then(status => sendResponse(status))
                .catch(error => sendResponse({ success: false, error: error.message }));
            return true; // 异步响应
        case 'getStatus':
            const response = {
                success: true,
                status: connectionStatus
            };
            sendResponse(response);
            return true; // 异步响应
        case 'captureTab':
            captureCurrentTab(message.options)
                .then(result => sendResponse({ success: true, result }))
                .catch(error => sendResponse({ success: false, error: error.message }));
            return true; // 异步响应
        case 'extractContent':
            extractTabContent(message.options)
                .then(result => sendResponse({ success: true, result }))
                .catch(error => sendResponse({ success: false, error: error.message }));
            return true; // 异步响应
        case 'settings_updated':
            // 设置已更新，直接调用WebSocketManager的updateConfig方法
            // 这样可以确保当配置被清空时，WebSocket连接会立即断开
            websocketManager.updateConfig();
            sendResponse({ success: true });
            break;
        default:
            sendResponse({ success: false, error: '未知操作' });
    }
}

// 连接到服务器
async function connectToServer() {
    try {
        await websocketManager.connect();
        // 获取实际的连接状态
        const status = websocketManager.getStatus();
        logger.log('connection', 'debug', 'connectToServer状态', { status });

        // 检查连接状态
        if (status.status === 'connected') {
            return { success: true };
        } else if (status.status === 'error') {
            return { success: false, error: '连接失败: ' + (status.errorMessage || '未知错误') };
        } else {
            // 如果状态不是connected，但也没有明确的错误
            return { success: false, error: '连接状态: ' + status.status };
        }
    } catch (error) {
        logger.error('connection', '连接服务器失败', { error: error.message });
        return { success: false, error: error.message };
    }
}

// 断开服务器连接
async function disconnectFromServer() {
    try {
        await websocketManager.disconnect();
        return { success: true };
    } catch (error) {
        logger.error('connection', '断开连接失败', { error: error.message });
        return { success: false, error: error.message };
    }
}

// 状态到图标文件的映射
const STATUS_ICON_MAP = {
    connected: 'connected',
    error: 'error',
    disconnected: 'disconnected',
    connecting: 'disconnected'
};

// 更新工具栏图标
function updateToolbarIcon(status) {
    const iconSuffix = STATUS_ICON_MAP[status] || 'disconnected';
    const iconPaths = {
        '16': `icons/icon16_${iconSuffix}.png`,
        '48': `icons/icon48_${iconSuffix}.png`,
        '128': `icons/icon128_${iconSuffix}.png`
    };
    chrome.action.setIcon({ path: iconPaths });
}

// 处理WebSocket连接状态变化
function handleConnectionStatusChange(status) {
    // 确保status对象包含所需属性
    if (!status) status = {};

    connectionStatus = {
        status: status.status || (status.connected ? 'connected' : 'disconnected'), // 确保status属性存在
        connected: status.connected || status.status === 'connected',
        serverUrl: status.serverUrl || websocketManager.serverUrl,
        lastConnected: (status.connected || status.status === 'connected') ? new Date().toISOString() : connectionStatus.lastConnected,
        lastError: status.error || status.errorMessage || null
    };

    // 更新工具栏图标以反映连接状态
    updateToolbarIcon(connectionStatus.status);

    // 通知弹出窗口连接状态变化
    try {
        chrome.runtime.sendMessage({
            action: 'connectionStatusChanged',
            status: connectionStatus
        }).catch(() => {
            // 弹出窗口可能未打开，忽略错误
            logger.log('background', 'debug', '弹出窗口未打开，忽略状态更新消息');
        });
    } catch (error) {
        // 静默处理发送消息失败的情况
        logger.log('background', 'error', '发送状态更新消息失败', { error: error.message });
    }
}

// 处理接收到的WebSocket消息
async function handleWebSocketMessage(message) {
    logger.log('background', 'debug', '收到WebSocket消息', { message });

    try {
        // 根据消息格式处理
        let data;
        if (message && message.data) {
            // 检查是否是从websocket.js传递过来的包装消息
            if (message.data && message.data.command && message.originalEvent) {
                data = message.data; // 直接使用包装中的data字段
            }
            // 如果消息是对象且包含data属性
            else if (typeof message.data === 'string') {
                // 如果data是字符串，尝试解析JSON
                try {
                    data = JSON.parse(message.data);
                } catch (error) {
                    logger.error('websocket', '解析JSON失败', { error: error.message });
                    data = message.data; // 如果解析失败，使用原始字符串
                }
            } else if (typeof message.data === 'object') {
                // 如果data已经是对象，直接使用
                logger.log('background', 'debug', 'handleWebSocketMessage -> data是对象，直接使用', { message });
                data = message.data;
            }
        } else if (typeof message === 'object') {
            // 消息本身可能就是数据
            logger.log('background', 'debug', 'handleWebSocketMessage -> 消息本身作为数据使用', { message });
            data = message;
        }

        // 确保data是有效的
        if (!data || typeof data !== 'object') {
            logger.warn('websocket', '收到无效消息格式', { message });
            return;
        }

        logger.log('background', 'debug', 'handleWebSocketMessage -> 处理后的data', { data });

        // 检查消息来源是否为ws_command
        if (data.source === 'ws_command') {
            logger.log('background', 'debug', '收到来自websocket_send_command的消息', { data });
            // 处理来自websocket_send_command的消息
            logger.log('background', 'debug', '收到websocket_send_command指令', { action: data.action, command: data.command, url: data.url });
            if (data.command === 'open' && data.url) {
                await handleCaptureCommand(data);
                return;
            }

            const responseMessage = {
                type: 'response',
                message_id: data.message_id,
                success: true,
                content: data // 将原始消息内容一并返回
            };
            // 返回默认消息回服务器
            logger.log('background', 'debug', '返回默认消息回服务器', { responseMessage });
            websocketManager.sendMessage(responseMessage);
            return;
        }

        // 处理标准指令格式的消息
        const command = data.command;
        logger.debug('background', '检查command字段', { command });
        if (command) {
            logger.debug('background', '处理command', { command });
            switch (command) {
                case 'capture':
                    logger.debug('background', '执行capture命令');
                    await handleCaptureCommand(data);
                    break;
                case 'extract':
                    logger.debug('background', '执行extract命令');
                    await handleExtractCommand(data);
                    break;

                case 'navigate':
                    logger.debug('background', '执行navigate命令');
                    await handleNavigateCommand(data);
                    break;

                case 'open':
                    logger.debug('background', '执行open命令');
                    await handleOpenCommand(data);
                    break;

                case 'intercept':
                    logger.debug('background', '执行intercept命令');
                    await handleInterceptCommand(data);
                    break;

                case 'runPageScript':
                    logger.debug('background', '执行runPageScript命令');
                    await handleRunPageScriptCommand(data);
                    break;

                case 'fetchInPage':
                    logger.debug('background', '执行fetchInPage命令');
                    await handleFetchInPageCommand(data);
                    break;

                default:
                    const responseMessage = {
                        type: 'response',
                        message_id: data.message_id,
                        success: false,
                        error: `未知指令: ${command}`,
                        content: data // 将原始消息内容一并返回
                    };
                    logger.warn('websocket', '未知指令', { command });
                    websocketManager.sendMessage(responseMessage);
            }
        } else {
            logger.warn('websocket', '消息中没有command字段', { data });
        }
    } catch (error) {
        logger.error('websocket', '处理WebSocket消息失败', { error: error.message });
    }
}

// 处理截图指令
async function handleCaptureCommand(data) {
    logger.log('capture', 'info', '开始处理截图指令', data);
    try {
        let targetTabId = null;

        // 如果指定了URL，先导航到该URL
        if (data.url) {
            logger.log('capture', 'info', '准备导航到URL', { url: data.url });
            const tab = await navigateToUrl(data.url);
            targetTabId = tab.id;
            logger.log('capture', 'info', 'URL导航完成', { tabId: targetTabId });
        }

        // 执行截图
        const captureOptions = {
            fullPage: data.fullPage !== false,
            area: data.area || null
        };
        logger.log('capture', 'info', '准备执行截图', captureOptions);

        const captureResult = await captureCurrentTab(captureOptions, targetTabId);
        const responseMessage = {
            success: true, type: 'response',
            message_id: data.message_id,
            command: 'capture',
            success: true,
            result: captureResult
        }
        // 发送结果回服务器
        logger.log('capture', 'info', '截图完成,准备发送截图结果到服务器', responseMessage);
        websocketManager.sendMessage(responseMessage);
    } catch (error) {
        // 记录错误日志
        logger.error('capture', '截图失败', { error: error.message });
        // 发送错误回服务器
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'capture',
            success: false,
            error: error.message
        });
    }
}

// 处理内容提取指令
async function handleExtractCommand(data) {
    let targetTabId = null;
    let shouldCloseTab = false;

    try {
        logger.log('extract', 'info', '开始处理内容提取指令', { message_id: data.message_id, url: data.url });

        // 如果指定了URL，先导航到新标签页
        if (data.url) {
            logger.log('extract', 'info', '准备导航到URL', { url: data.url });
            const tab = await navigateToUrl(data.url);
            targetTabId = tab.id;
            shouldCloseTab = true;
            logger.log('extract', 'info', 'URL导航完成', { tabId: targetTabId });
        }

        // 执行内容提取（用导航返回的 tabId，避免用户切走标签页导致提取错误页面）
        const extractOptions = {
            extractImages: data.extractImages !== false,
            extractLinks: data.extractLinks !== false,
            customSelectors: data.selectors || null
        };

        logger.log('extract', 'info', '准备执行内容提取', { tabId: targetTabId, ...extractOptions });
        const extractResult = await extractTabContent(extractOptions, targetTabId);
        logger.log('extract', 'info', '内容提取完成', { result: extractResult });

        // 发送结果回服务器
        logger.log('extract', 'info', '准备发送提取结果到服务器');
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'extract',
            success: true,
            result: extractResult
        });
        logger.log('extract', 'info', '提取结果已发送到服务器');
    } catch (error) {
        logger.error('extract', '内容提取失败', { error: error.message });

        // 发送错误回服务器
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'extract',
            success: false,
            error: error.message
        });
        logger.log('extract', 'error', '错误信息已发送到服务器');
    } finally {
        // 如果是新打开的标签页，提取完成后自动关闭
        if (shouldCloseTab && targetTabId !== null) {
            try {
                await chrome.tabs.remove(targetTabId);
                logger.log('extract', 'info', '已关闭临时标签页', { tabId: targetTabId });
            } catch (e) {
                logger.log('extract', 'warn', '关闭临时标签页失败', { tabId: targetTabId, error: e.message });
            }
        }
    }
}

// 处理导航指令
async function handleNavigateCommand(data) {
    try {
        if (!data.url) {
            throw new Error('URL不能为空');
        }

        await navigateToUrl(data.url);

        // 发送结果回服务器
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'navigate',
            success: true,
            url: data.url
        });
    } catch (error) {
        // 发送错误回服务器
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'navigate',
            success: false,
            error: error.message
        });
    }
}

// 处理打开URL指令 (来自websocket_send_command)
async function handleOpenCommand(data) {
    try {
        if (!data.url) {
            throw new Error('URL不能为空');
        }

        await navigateToUrl(data.url);

        // 发送结果回服务器
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id || 'open_' + Date.now(), // 确保有ID，即使原始消息没有提供
            command: 'open',
            success: true,
            url: data.url
        });
    } catch (error) {
        // 发送错误回服务器
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id || 'open_' + Date.now(),
            command: 'open',
            success: false,
            error: error.message
        });
    }
}

// ===== intercept 命令：早注入捕获页面 XHR/GraphQL 响应 =====
// 设计见 docs/browser-executor-plan.md §4.3。
// hook(MAIN world) 捕获响应 -> postMessage -> bridge(ISOLATED) 缓冲 ->
// background flush 取回 -> 回传服务端，服务端适配器按 operation 名过滤解析。

// 确保 document_start 捕获脚本已为目标 host 注册（幂等）
async function ensureInterceptScriptsRegistered(targetUrl) {
    let host;
    try {
        host = new URL(targetUrl).hostname;
    } catch (_) {
        throw new Error('intercept: 非法 URL');
    }
    // 取主域，匹配子域（如 x.com / api.x.com）
    const parts = host.split('.');
    const base = parts.length >= 2 ? parts.slice(-2).join('.') : host;
    const matches = [`*://${base}/*`, `*://*.${base}/*`];
    const scriptId = `grabby-intercept-${base}`;

    const existing = await chrome.scripting.getRegisteredContentScripts({ ids: [scriptId + '-hook', scriptId + '-bridge'] }).catch(() => []);
    if (existing && existing.length >= 2) return; // 已注册

    // 先尝试反注册残留，避免重复注册报错
    try {
        await chrome.scripting.unregisterContentScripts({ ids: [scriptId + '-hook', scriptId + '-bridge'] });
    } catch (_) {}

    await chrome.scripting.registerContentScripts([
        {
            id: scriptId + '-hook',
            matches,
            js: ['content/intercept-hook.js'],
            runAt: 'document_start',
            world: 'MAIN'
        },
        {
            id: scriptId + '-bridge',
            matches,
            js: ['content/intercept-bridge.js'],
            runAt: 'document_start',
            world: 'ISOLATED'
        }
    ]);
    logger.log('intercept', 'info', '已注册捕获脚本', { base, matches });
}

// 向 bridge 请求当前缓冲的捕获并清空
function flushCaptures(tabId) {
    return new Promise((resolve) => {
        chrome.tabs.sendMessage(tabId, { type: 'grabby:flush', drain: true }, (resp) => {
            if (chrome.runtime.lastError || !resp || !resp.ok) {
                resolve([]);
                return;
            }
            resolve(resp.captures || []);
        });
    });
}

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

async function handleInterceptCommand(data) {
    let targetTabId = null;
    let previousActiveTabId = null;
    const params = data.params || {};
    const scrollRounds = Number.isInteger(params.scrollRounds) ? params.scrollRounds : 6;
    const scrollDelayMs = Number.isInteger(params.scrollDelayMs) ? params.scrollDelayMs : 1500;
    const maxCaptures = Number.isInteger(params.maxCaptures) ? params.maxCaptures : 200;
    const visible = data.visible === true;

    try {
        if (!data.url) throw new Error('intercept: URL 不能为空');
        logger.log('intercept', 'info', '开始 intercept', { url: data.url, visible, scrollRounds });

        await ensureInterceptScriptsRegistered(data.url);

        // 记录当前活动 tab，结束后可恢复
        if (visible) {
            const [cur] = await chrome.tabs.query({ active: true, currentWindow: true });
            if (cur) previousActiveTabId = cur.id;
        }

        // 建 tab。可见性依赖滚动懒加载的页面(timeline/likes)用 visible:true
        const tab = await navigateToUrl(data.url, visible);
        targetTabId = tab.id;

        // 滚动分页 + 分轮 flush，累积捕获
        const collected = [];
        // 首屏捕获
        collected.push(...await flushCaptures(targetTabId));

        for (let i = 0; i < scrollRounds && collected.length < maxCaptures; i++) {
            await scrollPage(targetTabId, 0.95); // 滚到接近底部触发下一页
            await sleep(scrollDelayMs);
            const batch = await flushCaptures(targetTabId);
            collected.push(...batch);
        }

        logger.log('intercept', 'info', 'intercept 完成', { count: collected.length });

        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'intercept',
            success: true,
            result: {
                url: data.url,
                timestamp: new Date().toISOString(),
                items: collected.slice(0, maxCaptures)
            }
        });
    } catch (error) {
        logger.error('intercept', 'intercept 失败', { error: error.message });
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'intercept',
            success: false,
            error: error.message
        });
    } finally {
        if (targetTabId !== null && (data.closeTab !== false)) {
            try { await chrome.tabs.remove(targetTabId); } catch (_) {}
        }
        if (previousActiveTabId !== null) {
            try { await chrome.tabs.update(previousActiveTabId, { active: true }); } catch (_) {}
        }
    }
}

// ===== runPageScript 命令：白名单页面脚本 =====
// 设计见 docs/browser-executor-plan.md §4.1。
// 只允许执行内置、静态的脚本，绝不接受任意 JS 字符串；脚本名走 name，
// 参数走结构化 args（禁止拼接进脚本体）。world:MAIN 才能读到页面 window 变量。
//
// 重要(MV3 陷阱)：executeScript 的 func 会被序列化后注入，访问不到
// service worker 侧的闭包/外部变量，因此白名单必须内联在函数体里用静态
// switch 分发。新增白名单脚本请直接加 case。
async function handleRunPageScriptCommand(data) {
    let targetTabId = null;
    try {
        if (!data.url) throw new Error('runPageScript: URL 不能为空');
        const scriptName = (data.params && data.params.script) || data.script;
        if (!scriptName) throw new Error('runPageScript: 缺少 script 名称');
        const scriptParams = (data.params && data.params.params) || {};
        logger.log('runPageScript', 'info', '开始执行白名单脚本', { url: data.url, script: scriptName });

        const fastInitialState = scriptName === 'xiaohongshu.userNotesInitialState';
        const tab = await navigateToUrl(data.url, data.visible === true, { waitForStable: !fastInitialState });
        targetTabId = tab.id;

        const [{ result } = {}] = await chrome.scripting.executeScript({
            target: { tabId: targetTabId },
            world: 'MAIN',
            // 注意：func 被序列化注入，访问不到外部闭包；白名单必须内联在函数体内。
            func: async (name, params) => {
                switch (name) {
                    case 'youtube.initialPlayerResponse':
                        return window.ytInitialPlayerResponse ?? null;
                    case 'bilibili.initialState':
                        return window.__INITIAL_STATE__ ?? null;
                    case 'xiaohongshu.initialState': {
                        // 小红书笔记详情走 SSR：数据在 window.__INITIAL_STATE__.note.noteDetailMap。
                        // 直接返回该对象（plain object）；structured clone + WS JSON 序列化会
                        // 自动省略值为 undefined 的属性，服务端拿到的 map 是干净的。
                        const st = window.__INITIAL_STATE__;
                        if (!st) return null;
                        try {
                            return st.note?.noteDetailMap ?? {};
                        } catch (_) {
                            return null;
                        }
                    }
                    case 'xiaohongshu.userNotesInitialState': {
                        const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
                        const findInitialStateScript = () => {
                            for (const script of document.scripts) {
                                const text = script.textContent || '';
                                if (text.includes('window.__INITIAL_STATE__')) return text;
                            }
                            return '';
                        };
                        const extractAssignedObject = (text) => {
                            const marker = 'window.__INITIAL_STATE__';
                            const markerIndex = text.indexOf(marker);
                            if (markerIndex < 0) return '';
                            const start = text.indexOf('{', markerIndex);
                            if (start < 0) return '';
                            let depth = 0;
                            let inString = false;
                            let escaped = false;
                            for (let i = start; i < text.length; i++) {
                                const ch = text[i];
                                if (inString) {
                                    if (escaped) {
                                        escaped = false;
                                    } else if (ch === '\\') {
                                        escaped = true;
                                    } else if (ch === '"') {
                                        inString = false;
                                    }
                                    continue;
                                }
                                if (ch === '"') {
                                    inString = true;
                                } else if (ch === '{') {
                                    depth++;
                                } else if (ch === '}') {
                                    depth--;
                                    if (depth === 0) return text.slice(start, i + 1);
                                }
                            }
                            return '';
                        };
                        const notesFromScript = () => {
                            const scriptText = findInitialStateScript();
                            const objectText = extractAssignedObject(scriptText);
                            if (!objectText) return { notes: [], source: 'script', scriptFound: !!scriptText, parseError: 'initial state object not found' };
                            try {
                                const undefinedCount = (objectText.match(/\bundefined\b/g) || []).length;
                                const normalized = objectText.replace(/\bundefined\b/g, 'null');
                                const parsed = JSON.parse(normalized);
                                return { notes: parsed?.user?.notes ?? [], source: 'script', scriptFound: true, undefinedCount };
                            } catch (e) {
                                return { notes: [], source: 'script', scriptFound: true, parseError: String((e && e.message) || e), prefix: objectText.slice(0, 200) };
                            }
                        };
                        for (let i = 0; i < 20; i++) {
                            const st = window.__INITIAL_STATE__;
                            try {
                                const notes = st?.user?.notes;
                                if (Array.isArray(notes) && notes.length > 0) {
                                    return { notes, __debug: { source: 'window', href: location.href, userKeys: Object.keys(st?.user || {}) } };
                                }
                            } catch (_) {}
                            const fromScript = notesFromScript();
                            if (Array.isArray(fromScript.notes) && fromScript.notes.length > 0) {
                                return { notes: fromScript.notes, __debug: { source: fromScript.source, href: location.href, scriptFound: fromScript.scriptFound, undefinedCount: fromScript.undefinedCount } };
                            }
                            await sleep(250);
                        }
                        const fromScript = notesFromScript();
                        return { notes: fromScript.notes, __debug: { source: fromScript.source, href: location.href, hasInitialState: !!window.__INITIAL_STATE__, userKeys: Object.keys(window.__INITIAL_STATE__?.user || {}), scriptFound: fromScript.scriptFound, parseError: fromScript.parseError, prefix: fromScript.prefix } };
                    }
                    case 'page.extractJsonLd': {
                        const items = [];
                        document.querySelectorAll('script[type="application/ld+json"]').forEach((s) => {
                            try { items.push(JSON.parse(s.textContent)); } catch (_) {}
                        });
                        return { items };
                    }
                    case 'page.readMeta': {
                        const meta = {};
                        document.querySelectorAll('meta[property], meta[name]').forEach((m) => {
                            const k = m.getAttribute('property') || m.getAttribute('name');
                            if (k && !(k in meta)) meta[k] = m.getAttribute('content');
                        });
                        return meta;
                    }
                    default:
                        return { __error: 'script not allowed: ' + name };
                }
            },
            args: [scriptName, scriptParams]
        });

        if (result && result.__error) throw new Error(result.__error);

        // 对象走 result.json；其余(数组/标量/null)序列化进 result.text。
        const isPlainObject = result !== null && typeof result === 'object' && !Array.isArray(result);
        const pageResult = isPlainObject
            ? { url: data.url, timestamp: new Date().toISOString(), json: result }
            : { url: data.url, timestamp: new Date().toISOString(), text: JSON.stringify(result ?? null) };

        if (scriptName === 'xiaohongshu.userNotesInitialState') {
            const groups = Array.isArray(result?.notes) ? result.notes : [];
            const sample = [];
            for (const group of groups) {
                const items = Array.isArray(group) ? group : [group];
                for (const item of items) {
                    if (!item || sample.length >= 3) break;
                    const card = item.noteCard || item.note_card || item;
                    sample.push({
                        id: item.id || card.noteId || card.note_id || card.id || '',
                        title: card.displayTitle || card.display_title || card.title || '',
                        type: card.type || '',
                        xsecToken: item.xsecToken || item.xsec_token || card.xsecToken || card.xsec_token || '',
                        author: card.user?.nickName || card.user?.nickname || card.user?.nick_name || '',
                        likedCount: card.interactInfo?.likedCount || card.interact_info?.liked_count || ''
                    });
                }
                if (sample.length >= 3) break;
            }
            logger.log('runPageScript', 'info', 'xiaohongshu.userNotesInitialState 结果', {
                groupCount: groups.length,
                firstGroupCount: Array.isArray(groups[0]) ? groups[0].length : (groups[0] ? 1 : 0),
                totalSampled: sample.length,
                sample,
                debug: result?.__debug
            });
        }

        logger.log('runPageScript', 'info', 'runPageScript 完成', { script: scriptName, hasResult: result != null });
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'runPageScript',
            success: true,
            result: pageResult
        });
    } catch (error) {
        logger.error('runPageScript', 'runPageScript 失败', { error: error.message });
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'runPageScript',
            success: false,
            error: error.message
        });
    } finally {
        if (targetTabId !== null && data.closeTab !== false) {
            try { await chrome.tabs.remove(targetTabId); } catch (_) {}
        }
    }
}

// ===== fetchInPage 命令：用页面上下文发 fetch（带登录态）=====
// 设计见 docs/browser-executor-plan.md §4.2。
// 在 MAIN world 里 fetch，同源、共享页面 Cookie/Origin，更接近真实请求；
// 但不万能——credentials/CORS/CSRF/动态 header 仍需调用方按站点显式处理。
//
// params:
//   requestUrl  实际请求地址(可与打开的 url 不同；省略时用 data.url)
//   method/headers/body/credentials  透传给 fetch 的选项
async function handleFetchInPageCommand(data) {
    let targetTabId = null;
    try {
        if (!data.url) throw new Error('fetchInPage: URL 不能为空');
        const p = data.params || {};
        const requestUrl = p.requestUrl || data.url;
        const fetchOpts = {
            method: p.method || 'GET',
            credentials: p.credentials || 'include'
        };
        if (p.headers && typeof p.headers === 'object') fetchOpts.headers = p.headers;
        if (p.body != null) fetchOpts.body = p.body;
        logger.log('fetchInPage', 'info', '开始页面内 fetch', { page: data.url, requestUrl, method: fetchOpts.method });

        const tab = await navigateToUrl(data.url, data.visible === true);
        targetTabId = tab.id;

        const [{ result } = {}] = await chrome.scripting.executeScript({
            target: { tabId: targetTabId },
            world: 'MAIN',
            func: async (target, opts) => {
                try {
                    const resp = await fetch(target, opts);
                    const body = await resp.text();
                    return { ok: true, status: resp.status, body };
                } catch (e) {
                    return { ok: false, error: String((e && e.message) || e) };
                }
            },
            args: [requestUrl, fetchOpts]
        });

        if (!result) throw new Error('fetchInPage: 页面内执行无返回');
        if (!result.ok) throw new Error('fetchInPage 请求失败: ' + result.error);

        logger.log('fetchInPage', 'info', 'fetchInPage 完成', { status: result.status, len: result.body ? result.body.length : 0 });
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'fetchInPage',
            success: true,
            result: {
                url: requestUrl,
                timestamp: new Date().toISOString(),
                text: result.body || '',
                json: { status: result.status }
            }
        });
    } catch (error) {
        logger.error('fetchInPage', 'fetchInPage 失败', { error: error.message });
        websocketManager.sendMessage({
            type: 'response',
            message_id: data.message_id,
            command: 'fetchInPage',
            success: false,
            error: error.message
        });
    } finally {
        if (targetTabId !== null && data.closeTab !== false) {
            try { await chrome.tabs.remove(targetTabId); } catch (_) {}
        }
    }
}

// 导航到指定URL
// 返回创建的 tab 对象，调用方可直接用 tab.id 提取内容
// active: 是否激活新标签页。现有命令不传 -> 保持旧行为(true)；
//         新命令(intercept 等)默认后台加载，避免抢用户焦点(见 plan §4.4)。
async function navigateToUrl(url, active = true, options = {}) {
    return new Promise((resolve, reject) => {
        try {
            chrome.tabs.create({ url, active }, (tab) => {
                if (chrome.runtime.lastError) {
                    reject(new Error(chrome.runtime.lastError.message));
                    return;
                }

                const listener = async (updatedTabId, changeInfo) => {
                    if (updatedTabId === tab.id && changeInfo.status === 'complete') {
                        chrome.tabs.onUpdated.removeListener(listener);
                        if (options.waitForStable === false) {
                            resolve(tab);
                            return;
                        }
                        // 页面框架加载完成，再等待内容稳定
                        try {
                            await waitForPageStable(tab.id);
                            resolve(tab);
                        } catch (e) {
                            resolve(tab); // 即使等待失败也继续
                        }
                    }
                };

                chrome.tabs.onUpdated.addListener(listener);

                // 30秒超时兜底
                setTimeout(() => {
                    chrome.tabs.onUpdated.removeListener(listener);
                    resolve(tab);
                }, 30000);
            });
        } catch (error) {
            reject(error);
        }
    });
}

// 等待页面内容稳定
// 通过轮询页面文本长度 + 滚动触发懒加载，直到内容连续稳定
async function waitForPageStable(tabId, maxWaitMs = 20000) {
    const checkInterval = 1500;
    const stableThreshold = 2; // 连续2次稳定
    const minContentLength = 100;
    const lengthTolerance = 50;

    let lastLength = 0;
    let stableCount = 0;
    let elapsed = 0;
    const scrollPositions = [0.3, 0.6, 0.9, 0.0]; // 30%, 60%, 90%, 回到顶部
    let scrollIndex = 0;

    return new Promise((resolve) => {
        const timer = setInterval(async () => {
            elapsed += checkInterval;

            // 滚动页面触发懒加载
            if (scrollIndex < scrollPositions.length) {
                await scrollPage(tabId, scrollPositions[scrollIndex]);
                scrollIndex++;
            }

            // 获取页面文本长度
            const currentLength = await getPageTextLength(tabId);
            const lengthDiff = Math.abs(currentLength - lastLength);

            if (lengthDiff < lengthTolerance && currentLength >= minContentLength) {
                stableCount++;
                logger.log('background', 'debug', '页面内容稳定', {
                    tabId, currentLength, lastLength, diff: lengthDiff, stableCount, elapsed
                });
                if (stableCount >= stableThreshold) {
                    clearInterval(timer);
                    resolve({ length: currentLength, elapsed });
                }
            } else {
                if (currentLength > 0) {
                    logger.log('background', 'debug', '页面内容变化', {
                        tabId, lastLength, currentLength, diff: lengthDiff, elapsed
                    });
                }
                stableCount = 0;
            }
            lastLength = currentLength;

            // 超时
            if (elapsed >= maxWaitMs) {
                clearInterval(timer);
                logger.log('background', 'warn', '页面加载超时', { tabId, length: currentLength, elapsed });
                resolve({ length: currentLength, elapsed });
            }
        }, checkInterval);
    });
}

// 滚动页面到指定位置（触发懒加载）
function scrollPage(tabId, ratio) {
    return new Promise((resolve) => {
        chrome.scripting.executeScript({
            target: { tabId },
            func: (scrollRatio) => {
                const scrollTo = scrollRatio * (document.body.scrollHeight || document.documentElement.scrollHeight);
                window.scrollTo({ top: scrollTo, behavior: 'smooth' });
            },
            args: [ratio]
        }, () => resolve());
    });
}

// 获取页面文本内容长度
function getPageTextLength(tabId) {
    return new Promise((resolve) => {
        chrome.scripting.executeScript({
            target: { tabId },
            func: () => {
                const body = document.body;
                if (!body) return 0;
                const text = body.innerText || body.textContent || '';
                return text.trim().length;
            }
        }, (results) => {
            if (results && results[0] && typeof results[0].result === 'number') {
                resolve(results[0].result);
            } else {
                resolve(0);
            }
        });
    });
}

// 截图指定标签页
// tabId: 可选，不传则取当前活动标签页
async function captureCurrentTab(options = {}, tabId = null) {
    try {
        let targetTab = null;
        if (tabId) {
            targetTab = await chrome.tabs.get(tabId);
        } else {
            const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
            targetTab = tab;
        }
        if (!targetTab) {
            throw new Error('无法获取当前标签页');
        }

        // 获取图片格式、质量和全页面截图设置
        const { imageFormat, imageQuality } = await new Promise(resolve => {
            chrome.storage.sync.get(['imageFormat', 'imageQuality'], (result) => {
                resolve({
                    imageFormat: result.imageFormat || 'png',
                    imageQuality: result.imageQuality || 90,
                });
            });
        });

        // 合并配置和传入的选项
        const captureOptions = {
            fullPage: options.fullPage,
            area: options.area || null,
            tabId: targetTab.id
        };

        const imageData = await captureManager.captureTab(captureOptions);

        return {
            url: targetTab.url,
            title: targetTab.title,
            timestamp: new Date().toISOString(),
            imageData,
            format: imageFormat,
            quality: imageQuality
        };
    } catch (error) {
        logger.error('capture', '截图失败', { error: error.message });
        throw error;
    }
}

// 提取指定标签页内容
// tabId: 可选，不传则取当前活动标签页
async function extractTabContent(options = {}, tabId = null) {
    try {
        let targetTabId = tabId;
        if (!targetTabId) {
            const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
            if (!tab) {
                throw new Error('无法获取当前标签页');
            }
            targetTabId = tab.id;
        }

        const content = await contentExtractor.extractContent(targetTabId, options);

        // 获取标签页最新信息
        const tab = await chrome.tabs.get(targetTabId);

        return {
            url: tab.url,
            title: tab.title,
            timestamp: new Date().toISOString(),
            content
        };
    } catch (error) {
        logger.error('extract', '内容提取失败', { error: error.message });
        throw error;
    }
}

// 初始化
init();
