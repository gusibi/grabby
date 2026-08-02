class WebSocketManager {
    constructor() {
        this.socket = null;
        this.serverUrl = null;
        this.apiToken = '';
        this.connId = null;
        this.browserName = '';
        this.reconnectAttempts = 0;
        this.reconnectInterval = 5000; // 固定 5 秒探测一次，服务器恢复后 5 秒内自动接上
        this.autoReconnect = true; // 只有配置错误 / 认证失败 / 用户主动断开时才置为 false
        this.connectInFlight = false; // 建连流程进行中标记
        this.listeners = {};
        this.connectionStatus = 'disconnected'; // 'connected', 'connecting', 'disconnected', 'error'
        this.errorMessage = null; // 存储最新的错误消息
        this.connectTimeoutTimer = null; // 连接超时计时器
        this.CONNECT_TIMEOUT_MS = 10000; // 10秒连接超时
        this.keepAliveInterval = 25000; // 25秒心跳间隔
        this.keepAliveTimer = null; // 心跳计时器
        this.PING_MESSAGE = JSON.stringify({ type: 'ping' }); // 心跳消息
        this.lastMessageAt = 0; // 最近一次收到服务器消息的时间
        this.pongSeen = false; // 服务器是否支持 pong（支持才启用心跳看门狗）

        // 从存储中加载配置
        this.loadConfig();

        // 监听配置变化
        chrome.storage.onChanged.addListener((changes, areaName) => {
            if (areaName === 'sync' && (changes.serverUrl || changes.browserName || changes.apiToken)) {
                console.log('检测到配置更改，将重新加载并连接...');
                this.updateConfig();
            }
        });
    }

    /**
     * 从Chrome存储中加载WebSocket配置
     */
    loadConfig() {
        return new Promise((resolve) => { // 返回 Promise 以便知道加载完成
            chrome.storage.sync.get(['serverUrl', 'browserName', 'apiToken'], (result) => {
                const oldServerUrl = this.serverUrl;
                const oldBrowserName = this.browserName;
                const oldAPIToken = this.apiToken;
                let configChanged = false;

                if (result.serverUrl && result.serverUrl !== oldServerUrl) {
                    this.serverUrl = result.serverUrl;
                    configChanged = true;
                } else if (!result.serverUrl) {
                    this.serverUrl = null;
                    if (oldServerUrl) configChanged = true;
                }

                this.browserName = (result.browserName || '').trim();
                if (this.browserName !== oldBrowserName) {
                    configChanged = true;
                }

                this.apiToken = (result.apiToken || '').trim();
                if (this.apiToken !== oldAPIToken) {
                    configChanged = true;
                }

                const finish = (browserConnectId) => {
                    this.connId = browserConnectId || null;
                    console.log('配置已加载:', { serverUrl: this.serverUrl, browserName: this.browserName, connId: this.connId, hasApiToken: !!this.apiToken });
                    resolve(configChanged);
                };

                chrome.storage.local.get(['browserConnectId'], (localResult) => {
                    finish(localResult.browserConnectId || null);
                });
            });
        });
    }

    /**
     * 更新WebSocket配置
     * 当检测到存储变化时调用此方法
     */
    async updateConfig() {
        console.log('开始更新WebSocket配置...');
        // 断开现有连接
        this.disconnect(true); // 传入 true 表示是配置更新导致的断开

        // 重新加载配置
        const changed = await this.loadConfig();

        // 如果配置有效且已更改或之前未连接，则尝试连接
        if (this.serverUrl && this.browserName) {
            console.log('配置有效，尝试连接...');
            this.connect();
        } else {
            console.log('配置无效或已清空，保持断开状态。');
            this.updateStatus('disconnected', '配置无效或未设置');
        }
    }

    /**
     * 建立WebSocket连接
     * 确保只创建一个WebSocket连接实例
     */
    async connect() {
        // 清除之前的连接超时计时器
        clearTimeout(this.connectTimeoutTimer);
        this.connectTimeoutTimer = null;

        // 检查是否已有活跃连接或正在连接
        if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
            console.log('已有活跃或正在连接的WebSocket，跳过连接');
            return;
        }

        // 建连过程中有 await（读配置、注册浏览器），需要额外的进行中标记，
        // 否则 Service Worker 重启和看门狗可能同时发起两次连接。
        if (this.connectInFlight) {
            console.log('已有连接流程正在进行，跳过本次连接');
            return;
        }
        this.connectInFlight = true;
        try {
            await this._doConnect();
        } finally {
            this.connectInFlight = false;
        }
    }

    async _doConnect() {

        // 显式发起连接时恢复自动重连
        this.autoReconnect = true;

        await this.loadConfig();

        // 检查配置（配置缺失属于不可自动恢复的错误，不重连）
        if (!this.serverUrl) {
            this.autoReconnect = false;
            this.updateStatus('error', '未配置服务器地址');
            return;
        }
        if (!this.browserName) {
            this.autoReconnect = false;
            this.updateStatus('error', '未配置浏览器名称');
            return;
        }
        if (!this.connId) {
            this.autoReconnect = false;
            this.updateStatus('error', '未设置浏览器连接标识，请在扩展设置页面中点击"随机生成"按钮生成');
            return;
        }

        // 验证 serverUrl 格式 (基本验证)
        let parsedUrl;
        try {
            parsedUrl = new URL(this.serverUrl);
            if (!['ws:', 'wss:'].includes(parsedUrl.protocol)) {
                const errorMsg = `无效的协议: ${parsedUrl.protocol}，请使用 ws:// 或 wss://`;
                this.autoReconnect = false; // 这是配置错误，不应该重连
                this.updateStatus('error', errorMsg);
                return;
            }
            if (!parsedUrl.hostname) {
                const errorMsg = `无效的主机名: ${this.serverUrl}`;
                this.autoReconnect = false;
                this.updateStatus('error', errorMsg);
                return;
            }
        } catch (e) {
            const errorMsg = `无效的 serverUrl 格式: ${this.serverUrl}`;
            this.autoReconnect = false; // 这是配置错误，不应该重连
            this.updateStatus('error', errorMsg);
            return;
        }

        this.updateStatus('connecting');
        console.log(`尝试连接到: ${parsedUrl.origin}${parsedUrl.pathname}?conn_id=${this.connId}`);

        try {
            await this.registerBrowser();

            // 创建WebSocket连接，将conn_id、name和api_key作为查询参数附加到URL
            const urlWithConnId = new URL(this.serverUrl);
            urlWithConnId.searchParams.append('conn_id', this.connId);
            urlWithConnId.searchParams.append('name', this.browserName);

            this.socket = new WebSocket(urlWithConnId.toString());

            // 设置连接超时
            this.connectTimeoutTimer = setTimeout(() => {
                if (this.socket && this.socket.readyState === WebSocket.CONNECTING) {
                    console.warn('WebSocket 连接超时');
                    // 主动关闭尝试超时的连接
                    this.socket.close(1000, 'Connection Timeout'); // 使用 1000 或自定义代码
                    // handleClose 会被触发，并处理重连逻辑
                }
            }, this.CONNECT_TIMEOUT_MS);

            // 设置事件处理器
            this.socket.onopen = this.handleOpen.bind(this);
            this.socket.onmessage = this.handleMessage.bind(this);
            this.socket.onclose = this.handleClose.bind(this);
            this.socket.onerror = this.handleError.bind(this); // onerror 通常在 onclose 之前触发

        } catch (error) {
            // 注册接口请求失败（服务器未启动/重启中）或 WebSocket 构造函数抛错
            console.error('建立连接失败:', error);
            this.socket = null; // 确保 socket 实例被清理
            this.updateStatus('disconnected', `连接失败: ${error.message}`);
            this.emit('disconnected', { message: error.message });
            // 服务器可能只是暂时不可用，继续按退避策略重试
            this.scheduleReconnect();
        }
    }

    /**
     * 连接 WebSocket 前注册当前浏览器实例
     */
    async registerBrowser() {
        const registerUrl = this.getRegisterUrl();
        const headers = { 'Content-Type': 'application/json' };
        if (this.apiToken) {
            headers['X-Grabby-Token'] = this.apiToken;
        }

        // 加超时，避免服务器无响应时 fetch 长时间挂起，卡住后续的 5 秒探测
        const abort = new AbortController();
        const abortTimer = setTimeout(() => abort.abort(), this.CONNECT_TIMEOUT_MS);
        let response;
        try {
            response = await fetch(registerUrl, {
                method: 'POST',
                headers,
                signal: abort.signal,
                body: JSON.stringify({
                    connect_id: this.connId,
                    name: this.browserName
                })
            });
        } catch (error) {
            if (error.name === 'AbortError') {
                throw new Error('浏览器注册超时，服务器无响应');
            }
            throw error;
        } finally {
            clearTimeout(abortTimer);
        }

        if (!response.ok) {
            let errorText = await response.text();
            try {
                const data = JSON.parse(errorText);
                errorText = data.detail || data.error || response.statusText;
            } catch (_) {}
            throw new Error(`浏览器注册失败: ${errorText}`);
        }
    }

    getRegisterUrl() {
        const url = new URL(this.serverUrl);
        url.protocol = url.protocol === 'wss:' ? 'https:' : 'http:';
        url.pathname = '/api/browsers/register';
        url.search = '';
        return url.toString();
    }

    /**
     * 处理WebSocket连接成功事件
     */
    handleOpen(event) {
        clearTimeout(this.connectTimeoutTimer); // 清除连接超时计时器
        this.connectTimeoutTimer = null;
        console.log('WebSocket连接已建立');
        this.updateStatus('connected');
        this.reconnectAttempts = 0; // 重置重连尝试次数

        // 启动心跳机制
        this.startKeepAlive();

        // 触发连接事件
        this.emit('connected');
    }

    /**
     * 处理接收到的WebSocket消息
     */
    handleMessage(event) {
        console.log('[WebSocketManager] 收到WebSocket消息:', event.data); // 调试日志
        this.lastMessageAt = Date.now(); // 供心跳看门狗判断连接是否存活
        try {
            if (!event || !event.data) {
                console.warn('收到空的WebSocket消息事件');
                return;
            }

            let message;
            try {
                message = JSON.parse(event.data);
            } catch (parseError) {
                console.error('解析WebSocket消息失败:', parseError, '原始数据:', event.data);
                this.emit('message', { rawData: event.data, parseError: true });
                return;
            }

            // 处理心跳响应
            if (message && typeof message === 'object' && message.type === 'pong') {
                this.pongSeen = true; // 服务器支持心跳应答，启用看门狗
                return;
            }

            // 处理认证响应
            if (message && typeof message === 'object' && message.type === 'auth_response') {
                this.handleAuthResponse(message);
                return;
            }

            // 触发通用消息事件
            this.emit('message', { data: message, originalEvent: event });

        } catch (error) {
            console.error('处理WebSocket消息时发生内部错误:', error);
            this.emit('error', { error, source: 'message_handler' });
        }
    }

    /**
     * 处理认证响应
     */
    handleAuthResponse(message) {
        if (message.success) {
            console.log('服务器认证成功。');
            // 如果服务器返回了确认的conn_id，可以更新它，虽然通常它就是apiKey
            if (message.conn_id && message.conn_id !== this.connId) {
                console.log(`服务器确认的 conn_id 不同: ${message.conn_id} (本地: ${this.connId})，已更新。`);
                this.connId = message.conn_id;
            }
            this.emit('auth_success', { connId: this.connId });
        } else {
            const errorMsg = `认证失败: ${message.error || '未知原因'}`;
            console.error(errorMsg);
            this.updateStatus('error', errorMsg);
            this.emit('auth_failure', message.error);
            // 认证失败通常意味着apiKey错误，阻止重连
            this.autoReconnect = false;
            this.disconnect(false); // 主动断开
        }
    }

    /**
     * 辅助方法：尝试推断是否为连接握手阶段的错误（可能包括403）
     * @param {CloseEvent} event - WebSocket 关闭事件
     * @returns {boolean} 是否可能是握手错误
     */
    isHandshakeError(event) {
        // 1. 检查明确的 "Forbidden" 或 "403" (虽然不常见)
        const reason = event?.reason?.toLowerCase() || '';
        if (reason.includes('forbidden') || reason.includes('403')) {
            console.warn('检测到关闭原因包含 "forbidden" 或 "403"');
            return true;
        }

        // 2. 检查常见的握手失败代码
        // 1006 (Abnormal Closure) 是最常见的，但也可能由其他原因引起
        // 1002 (Protocol Error)
        // 1015 (TLS Handshake failure) - 通常是 WSS 配置问题
        if ([1002, 1006, 1015].includes(event?.code)) {
            // 如果状态仍然是 'connecting' 就关闭，极有可能是握手失败
            if (this.connectionStatus === 'connecting') {
                console.warn(`连接在 "connecting" 状态下关闭 (Code: ${event.code})，推测为握手失败。`);
                return true;
            }
            // 如果是 1015 TLS 错误，几乎肯定是配置/服务器问题
            if (event.code === 1015) {
                console.warn(`检测到 TLS 握手失败 (Code: 1015)`);
                return true;
            }
            // 对于 1006，如果不是在 connecting 状态，则不太确定
            if (event.code === 1006) {
                console.log(`连接异常关闭 (Code: 1006)，原因未知，可能不是握手错误。`);
            }
        }

        // 3. 检查连接超时关闭（ ourselves 触发的）
        if (event?.code === 1000 && reason === 'Connection Timeout') {
            console.warn('检测到连接超时关闭。');
            return true; // 视为一种连接建立失败
        }

        return false;
    }


    /**
     * 处理WebSocket关闭事件
     */
    handleClose(event) {
        clearTimeout(this.connectTimeoutTimer); // 清除可能存在的连接超时计时器
        this.connectTimeoutTimer = null;

        console.log(`WebSocket连接已关闭: 代码=${event.code}, 原因='${event.reason || '无'}' Clean=${event.wasClean}`);

        // 如果 socket 实例已被手动置 null (例如在 disconnect 中)，则不再处理
        if (!this.socket) {
            console.log("Socket 实例已清除，跳过 handleClose 处理。");
            return;
        }

        this.socket = null; // 清理 socket 实例引用

        const wasConnecting = this.connectionStatus === 'connecting';
        const possibleHandshakeError = this.isHandshakeError(event);

        // 默认错误消息和重连决策
        let errorMsg = null;
        let shouldReconnect = true;
        let finalStatus = 'disconnected'; // 默认为断开

        if (event.wasClean && event.code === 1000 && this.statusBeforeDisconnect) {
            // 我们自己发起的正常关闭（手动断开 / 配置更新），不重连
            console.log("WebSocket 由本地主动正常关闭。");
            shouldReconnect = false;
        } else if (event.wasClean && event.code === 1000) {
            // 服务器主动正常关闭（例如优雅停机），应该重连
            console.log("服务器主动关闭了连接，将尝试重连。");
            errorMsg = '服务器关闭了连接。';
            shouldReconnect = true;
        } else if (possibleHandshakeError) {
            // 推测为握手错误 (可能 403, 404, URL错误, TLS错误, 超时等)
            // 注意：服务器停止 / 重启中同样表现为握手失败，因此这里必须继续重试，
            // 只是把失败原因展示出来，交给指数退避控制频率。
            finalStatus = 'disconnected';
            if (event.code === 1015) {
                errorMsg = "连接失败: TLS 握手错误。请检查wss://地址和服务器证书。";
            } else if (event.reason && (event.reason.toLowerCase().includes('forbidden') || event.reason.includes('403'))) {
                errorMsg = "连接失败: 服务器拒绝连接 (403 Forbidden)。请检查浏览器注册状态或服务器权限设置。";
            } else if (event.reason === 'Connection Timeout') {
                errorMsg = "连接失败: 连接超时。请检查服务器地址和网络。";
            } else {
                // 通用握手错误消息
                errorMsg = `连接失败 (Code: ${event.code})。请检查服务器地址 (${this.serverUrl}) 是否正确，以及服务器是否正在运行。`;
            }
            shouldReconnect = true; // 服务器可能只是暂时不可用，继续退避重试
            console.warn(`连接建立失败: ${errorMsg}，将按退避策略重试。`);
        } else {
            // 其他异常关闭 (例如网络中断、服务器崩溃后未正常关闭连接)
            finalStatus = 'disconnected'; // 状态是断开，但会尝试重连
            errorMsg = `连接意外断开 (Code: ${event.code})。`;
            console.warn(errorMsg, '将尝试重新连接。');
            shouldReconnect = true; // 尝试重连
        }

        // 更新状态
        this.updateStatus(finalStatus, errorMsg);
        this.emit(finalStatus === 'error' ? 'error' : 'disconnected', { code: event.code, reason: event.reason, wasClean: event.wasClean, message: errorMsg });


        // 根据情况决定是否重连
        if (shouldReconnect && this.autoReconnect && this.connectionStatus !== 'connected') {
            this.scheduleReconnect();
        } else if (!shouldReconnect || !this.autoReconnect) {
            console.log("根据关闭事件分析，不安排自动重连。");
        }

        // 清理标记
        this.statusBeforeDisconnect = null;
    }

    /**
     * 处理WebSocket错误事件
     * 注意：onerror 事件通常在 onclose 之前触发，且信息可能很有限。
     * 主要依赖 onclose 来判断连接失败的原因。
     */
    handleError(error) {
        // 这个事件通常只提供非常有限的信息，例如 "Error in connection establishment: net::ERR_NAME_NOT_RESOLVED"
        // 或者干脆就是一个通用的 "Event" 对象。
        // 它本身很难用来判断 403。
        console.error('WebSocket onerror 事件触发:', error);

        // 通常 onerror 后会立即触发 onclose，我们将主要逻辑放在 onclose 中处理。
        // 这里可以记录下错误，但不必立即更新状态或决定重连，等待 onclose 提供更详细信息。
        // 可以在这里暂存一个可能的错误信息，如果 onclose 没有提供更好的信息时使用。
        this.potentialErrorMessage = "WebSocket 发生未知连接错误。";
    }

    /**
     * 安排重新连接
     */
    scheduleReconnect() {
        if (this.connectionStatus === 'connecting' || (this.socket && this.socket.readyState === WebSocket.OPEN)) {
            console.log("已经在连接或已连接，取消重连调度。");
            return;
        }

        if (!this.autoReconnect) {
            console.log("自动重连已禁用（用户主动断开或配置/认证错误），不安排重连。");
            return;
        }

        this.reconnectAttempts++;
        // 固定间隔轮询，不做指数退避、不设最大次数：
        // 服务器可能长时间不可用，恢复后必须在一个探测周期内自动接上。
        const delay = this.reconnectInterval;
        console.log(`${delay / 1000}秒后尝试重新连接 (第 ${this.reconnectAttempts} 次)`);

        // 清除可能存在的旧计时器
        clearTimeout(this.reconnectTimer);

        this.reconnectTimer = setTimeout(() => {
            if (this.connectionStatus !== 'connected' && this.connectionStatus !== 'connecting') {
                console.log("执行重连...");
                this.connect();
            } else {
                console.log("重连计时器触发，但状态已变为连接或正在连接，取消本次重连。");
            }
        }, delay);
    }

    /**
     * 断开WebSocket连接
     * @param {boolean} isConfigUpdate - 是否因为配置更新而断开
     */
    disconnect(isConfigUpdate = false) {
        clearTimeout(this.reconnectTimer); // 清除重连计时器
        clearTimeout(this.connectTimeoutTimer); // 清除连接超时计时器
        clearInterval(this.keepAliveTimer); // 清除心跳计时器
        this.reconnectTimer = null;
        this.connectTimeoutTimer = null;
        this.keepAliveTimer = null;

        if (this.socket) {
            console.log(`主动断开WebSocket连接，连接ID: ${this.connId}`);
            // 设置一个标记，以便 handleClose 知道是主动断开还是配置更新
            this.statusBeforeDisconnect = isConfigUpdate ? 'config_update' : 'manual_disconnect';

            // 设置 onclose 为 null，避免在手动关闭时触发重连逻辑 (可选，但更清晰)
            // this.socket.onclose = null;
            // this.socket.onerror = null;

            if (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING) {
                this.socket.close(1000, isConfigUpdate ? "Configuration updated" : "Manual disconnection"); // 使用 1000 表示正常关闭
            }
            this.socket = null; // 立即清除引用
        } else {
            console.log("没有活跃的 WebSocket 连接可断开。");
        }

        // 如果不是因为配置更新，则更新状态为 disconnected 并停止自动重连
        if (!isConfigUpdate) {
            this.autoReconnect = false; // 用户主动断开，后台看门狗也不应再拉起连接
            this.updateStatus('disconnected');
        }
        // 清空 connId 可能不是最佳选择，因为重连时还需要它。
        // 可以在连接失败或成功获取新 ID 时再更新 connId。
        // this.connId = null;
        this.reconnectAttempts = 0; // 重置重连次数
    }

    /**
     * 启动心跳机制
     */
    startKeepAlive() {
        if (this.keepAliveTimer) {
            clearInterval(this.keepAliveTimer);
        }
        this.lastMessageAt = Date.now();
        this.keepAliveTimer = setInterval(() => {
            if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
                return;
            }

            // 看门狗：服务器支持 pong 时，若连续两个心跳周期收不到任何消息，
            // 说明连接已经"假死"（服务器进程被杀、网络中断但 close 事件未到达），
            // 主动关闭以触发重连。
            if (this.pongSeen && Date.now() - this.lastMessageAt > this.keepAliveInterval * 2 + 5000) {
                console.warn('心跳超时，判定连接已失效，主动重连。');
                try {
                    this.socket.close(4000, 'Heartbeat timeout');
                } catch (_) {}
                // close 事件不一定可靠触发，这里直接兜底走重连流程
                this.socket = null;
                clearInterval(this.keepAliveTimer);
                this.keepAliveTimer = null;
                this.updateStatus('disconnected', '心跳超时，连接已断开。');
                this.emit('disconnected', { code: 4000, reason: 'Heartbeat timeout' });
                this.scheduleReconnect();
                return;
            }

            try {
                this.socket.send(this.PING_MESSAGE);
                console.log('发送心跳消息');
            } catch (error) {
                console.warn('发送心跳失败，将重连:', error);
            }
        }, this.keepAliveInterval);
    }

    /**
     * 发送消息到服务器
     */
    sendMessage(message) {
        if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
            console.error('WebSocket未连接，无法发送消息:', message);
            return false;
        }

        try {
            const messageStr = typeof message === 'string' ? message : JSON.stringify(message);
            // console.log('发送消息:', messageStr); // 调试时打开
            this.socket.send(messageStr);
            return true;
        } catch (error) {
            console.error('发送消息失败:', error);
            // 可以在这里触发一个错误事件或尝试处理
            this.handleError(new Error(`Failed to send message: ${error.message}`));
            return false;
        }
    }

    /**
     * 更新连接状态并通知其他部分
     */
    updateStatus(status, errorMessage = null) {
        // 只有当状态实际改变时才执行更新和通知
        if (status === this.connectionStatus && errorMessage === this.errorMessage) {
            return;
        }

        this.connectionStatus = status;

        // 如果状态变为非错误状态，清除错误消息
        if (status === 'connected' || status === 'disconnected' || status === 'connecting') {
            // 但如果传入了错误消息（例如 'disconnected' 时附带原因），则保留
            this.errorMessage = errorMessage;
        } else if (status === 'error') {
            // 如果状态是 error，优先使用传入的 errorMessage，否则保留旧的或使用默认值
            this.errorMessage = errorMessage || this.errorMessage || '未知错误';
        }

        const statusObj = {
            status: this.connectionStatus,
            errorMessage: this.errorMessage,
            connected: this.connectionStatus === 'connected',
            serverUrl: this.serverUrl,
            connId: this.connId,
            browserName: this.browserName
        };

        console.log('WebSocket 状态更新:', statusObj);

        // 触发内部状态变更事件
        this.emit('status_change', statusObj);

        // 尝试发送消息到扩展的其他部分 (例如 popup 或 options 页面)
        this.notifyExtension('connection_status_update', statusObj);
    }

    /**
     * 向扩展的其他部分发送消息 (Service Worker 安全)
     */
    notifyExtension(action, data) {
        const message = { action, ...data };
        // 尝试发送给 Popup (如果打开)
        chrome.runtime.sendMessage(message).catch(error => {
            // 忽略 "Could not establish connection. Receiving end does not exist." 错误
            if (!error.message.includes('Receiving end does not exist')) {
                console.warn('向运行时发送消息失败 (可能是 popup 未打开):', error);
            }
        });

        // 如果需要通知所有 Content Scripts (通常不需要通知状态)
        // chrome.tabs.query({}, (tabs) => {
        //     tabs.forEach(tab => {
        //         chrome.tabs.sendMessage(tab.id, message).catch(error => {
        //             if (!error.message.includes('Receiving end does not exist')) {
        //                  console.warn(`向 Tab ${tab.id} 发送消息失败:`, error);
        //             }
        //         });
        //     });
        // });
    }


    /**
     * 获取当前连接状态
     */
    getStatus() {
        return {
            status: this.connectionStatus,
            serverUrl: this.serverUrl,
            connId: this.connId,
            browserName: this.browserName,
            errorMessage: this.errorMessage
        };
    }

    /**
     * 注册事件监听器
     */
    on(event, callback) {
        if (!this.listeners[event]) {
            this.listeners[event] = [];
        }
        this.listeners[event].push(callback);
        return this;
    }

    /**
     * 移除事件监听器
     */
    off(event, callback) {
        if (!this.listeners[event]) return this;
        if (callback) {
            this.listeners[event] = this.listeners[event].filter(cb => cb !== callback);
        } else {
            delete this.listeners[event];
        }
        return this;
    }

    /**
     * 触发事件
     */
    emit(event, data) {
        console.log(`WebSocketManager.emit: 触发事件 '${event}'`, data ? JSON.stringify(data).substring(0, 100) + '...' : '');
        if (!this.listeners[event] || this.listeners[event].length === 0) {
            // console.log(`WebSocketManager.emit: 事件 '${event}' 没有监听器。`);
            // 取消了默认响应逻辑，因为这可能导致意外行为
            return;
        }

        // 使用 slice() 创建副本，防止监听器在迭代过程中修改数组导致问题
        this.listeners[event].slice().forEach(callback => {
            try {
                callback(data);
            } catch (error) {
                console.error(`执行事件 '${event}' 的监听器时出错:`, error);
            }
        });
    }
}

// --- Service Worker 环境下的实例化和导出 ---
// 注意：这里只创建唯一实例。以前这里 new 一次、background.js 又 new 一次，
// 会产生两个互相不知道对方存在的管理器（各自监听 storage 变化、各自建连），
// 导致重复注册和 conn_id 抢占。

if (!self.websocketManagerInstance) {
    self.websocketManagerInstance = new WebSocketManager();
    console.log("WebSocketManager 已在 Service Worker 中实例化。");
}

self.WebSocketManager = WebSocketManager; // 导出类本身（如果需要）
self.websocketManager = self.websocketManagerInstance; // 导出唯一实例
