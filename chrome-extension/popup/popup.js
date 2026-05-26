/**
 * 弹出窗口脚本
 * 负责处理弹出窗口的用户界面交互
 */

// DOM 元素
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');
const serverInfo = document.getElementById('serverInfo');
const connectBtn = document.getElementById('connectBtn');
const disconnectBtn = document.getElementById('disconnectBtn');
const captureBtn = document.getElementById('captureBtn');
const extractBtn = document.getElementById('extractBtn');
const fullPageCapture = document.getElementById('fullPageCapture');
const resultPreview = document.getElementById('resultPreview');
const previewContent = document.getElementById('previewContent');
const saveResultBtn = document.getElementById('saveResultBtn');
const closePreviewBtn = document.getElementById('closePreviewBtn');
const openOptionsBtn = document.getElementById('openOptionsBtn');

// 当前结果数据
let currentResult = null;

// 初始化弹出窗口
function initPopup() {
    // 获取连接状态
    updateConnectionStatus();

    // 加载配置
    loadConfig();

    // 设置事件监听器
    setupEventListeners();

    // 监听连接状态变化消息
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (message.action === 'connectionStatusChanged') {
            updateConnectionUI(message.status);
        }
    });
}

// 加载配置
function loadConfig() {
    chrome.storage.sync.get(['serverUrl'], (result) => {
        if (result.serverUrl) {
            serverInfo.textContent = `服务器: ${result.serverUrl}`;
        }
    });
}

// 设置事件监听器
function setupEventListeners() {
    // 连接按钮
    connectBtn.addEventListener('click', () => {
        chrome.runtime.sendMessage({ action: 'connect' }, (response) => {
            console.log("response----->:", response)
            if (response && response.success) {
                // 连接成功，更新状态
                updateConnectionStatus();
            } else {
                // 连接失败，显示错误消息
                const errorMsg = response && response.error ? response.error : '连接服务器失败，请检查服务器配置';
                showError(errorMsg);
                updateConnectionStatus(); // 更新状态显示
            }
        });
    });

    // 断开按钮
    disconnectBtn.addEventListener('click', () => {
        chrome.runtime.sendMessage({ action: 'disconnect' }, (response) => {
            if (response && response.success) {
                updateConnectionStatus();
            }
        });
    });

    // 截图按钮
    captureBtn.addEventListener('click', captureCurrentTab);

    // 提取内容按钮
    extractBtn.addEventListener('click', extractCurrentTab);

    // 保存结果按钮
    saveResultBtn.addEventListener('click', saveResult);

    // 复制结果按钮
    document.getElementById('copyResultBtn').addEventListener('click', () => {
        if (currentResult) {
            if (currentResult.type === 'screenshot') {
                // 复制截图到剪贴板（注意：某些浏览器可能不支持直接复制图片）
                try {
                    navigator.clipboard.write([
                        new ClipboardItem({
                            'image/png': fetch(currentResult.dataUrl).then(r => r.blob())
                        })
                    ]).then(() => {
                        showSuccess('截图已复制到剪贴板！');
                    }).catch(err => {
                        showError('复制截图失败: ' + err.message);
                    });
                } catch (error) {
                    showError('您的浏览器不支持直接复制图片，请使用保存功能。');
                }
            } else if (currentResult.type === 'content') {
                // 复制内容到剪贴板
                copyContentToClipboard(currentResult.content);
            }
        }
    });

    // 关闭预览按钮
    closePreviewBtn.addEventListener('click', () => {
        resultPreview.style.display = 'none';
        currentResult = null;
    });

    // 打开选项页面按钮
    openOptionsBtn.addEventListener('click', () => {
        chrome.runtime.openOptionsPage();
    });

    // 打开日志页面按钮
    document.getElementById('openLogsBtn').addEventListener('click', () => {
        chrome.tabs.create({ url: chrome.runtime.getURL('logs/logs.html') });
    });
}

// 更新连接UI状态
function updateConnectionUI(status) {
    // 更新状态指示器
    statusDot.className = 'status-dot ' + status.status;

    // 更新状态文本
    switch (status.status) {
        case 'connected':
            statusText.textContent = '已连接';
            // 连接成功后，更新UI状态
            connectBtn.disabled = true;
            disconnectBtn.disabled = false;
            statusDot.className = 'status-dot connected';
            // 连接成功后，更新结果预览区域
            updateResultPreviewForConnection(true);
            break;
        case 'connecting':
            statusText.textContent = '连接中...';
            // 连接中，两个按钮都禁用
            connectBtn.disabled = true;
            disconnectBtn.disabled = true;
            break;
        case 'disconnected':
            statusText.textContent = '未连接';
            // 断开连接后，更新UI状态
            connectBtn.disabled = false;
            disconnectBtn.disabled = true;
            statusDot.className = 'status-dot disconnected';
            // 断开连接后，更新结果预览区域
            updateResultPreviewForConnection(false);
            break;
        case 'error':
            statusText.textContent = status.lastError || '连接错误';
            // 连接错误时，允许重新连接
            connectBtn.disabled = false;
            disconnectBtn.disabled = true;
            // 连接错误时，更新结果预览区域
            updateResultPreviewForConnection(false);
            break;
        default:
            statusText.textContent = '未知状态';
            // 未知状态时的默认UI
            connectBtn.disabled = false;
            disconnectBtn.disabled = true;
            // 未知状态时，更新结果预览区域
            updateResultPreviewForConnection(false);
    }

    // 更新服务器信息
    if (status.serverUrl) {
        serverInfo.textContent = `服务器: ${status.serverUrl}`;
    }
}

// 更新连接状态
function updateConnectionStatus() {
    chrome.runtime.sendMessage({ action: 'getStatus' }, (response) => {
        if (!response || !response.success) return;
        updateConnectionUI(response.status);
    });
}

// 捕获当前标签页截图
async function captureCurrentTab() {
    try {
        // 禁用按钮，显示加载状态
        captureBtn.disabled = true;
        captureBtn.textContent = '截图中.........';

        // 检查连接状态
        const isConnected = await checkConnectionStatus();

        if (isConnected) {
            // 连接到服务器时，发送截图请求到后台
            chrome.runtime.sendMessage({
                action: 'captureTab',  // 修正action名称与background.js匹配
                options: {
                    fullPage: fullPageCapture.checked
                }
            }, handleCaptureResponse);
        } else {
            // 未连接服务器时，直接在本地处理截图
            try {
                // 使用chrome.tabs API直接截图
                chrome.tabs.captureVisibleTab(null, { format: 'png' }, (dataUrl) => {
                    if (chrome.runtime.lastError) {
                        throw new Error(chrome.runtime.lastError.message);
                    }

                    // 处理截图结果
                    handleCaptureResponse({
                        success: true,
                        result: {
                            dataUrl: dataUrl,
                            timestamp: new Date().toISOString()
                        }
                    });

                    // 显示成功提示
                    showSuccess('截图成功！图片已保存在本地。');
                });
            } catch (error) {
                handleCaptureResponse({
                    success: false,
                    error: error.message
                });
            }
        }
    } catch (error) {
        // 恢复按钮状态
        captureBtn.disabled = false;
        captureBtn.innerHTML = '<span class="icon">📷</span><span class="label">截图当前页面</span>';

        showError(error.message);
    }
}

// 处理截图响应
function handleCaptureResponse(response) {
    // 恢复按钮状态
    captureBtn.disabled = false;
    captureBtn.innerHTML = '<span class="icon">📷</span><span class="label">截图当前页面</span>';

    if (!response || !response.success) {
        showError(response?.error || '截图失败');
        return;
    }

    // 显示截图结果
    currentResult = {
        type: 'screenshot',
        dataUrl: response.result.dataUrl
    };

    // 显示预览
    previewContent.innerHTML = `<img src="${response.result.dataUrl}" alt="页面截图">`;
    resultPreview.style.display = 'block';
}


// 提取当前标签页内容
async function extractCurrentTab() {
    try {
        // 禁用按钮，显示加载状态
        extractBtn.disabled = true;
        extractBtn.textContent = '提取中...';

        // 检查连接状态
        const isConnected = await checkConnectionStatus();

        if (isConnected) {
            // 连接到服务器时，发送内容提取请求到后台
            chrome.runtime.sendMessage({
                action: 'extractContent',  // 修正action名称与background.js匹配
                options: {}
            }, handleExtractResponse);
        } else {
            // 未连接服务器时，直接在本地处理内容提取
            try {
                // 使用chrome.scripting API在当前页面执行脚本提取内容
                chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
                    if (!tabs || !tabs[0]) {
                        throw new Error('无法获取当前标签页');
                    }

                    chrome.scripting.executeScript({
                        target: { tabId: tabs[0].id },
                        function: () => {
                            // 简单的内容提取函数
                            function extractPageContent() {
                                // 提取标题和URL
                                const title = document.title;
                                const url = window.location.href;

                                // 提取主要内容
                                let content = '';
                                const mainSelectors = ['article', '.article', '.post', '.content', 'main', '#content'];
                                for (const selector of mainSelectors) {
                                    const element = document.querySelector(selector);
                                    if (element) {
                                        content = element.innerText;
                                        break;
                                    }
                                }

                                // 如果没有找到主要内容，提取所有段落
                                if (!content) {
                                    const paragraphs = document.querySelectorAll('p');
                                    content = Array.from(paragraphs)
                                        .map(p => p.innerText.trim())
                                        .filter(text => text.length > 20)
                                        .join('\n\n');
                                }

                                // 提取图片
                                const images = Array.from(document.querySelectorAll('img'))
                                    .filter(img => img.width > 100 && img.height > 100) // 过滤小图标
                                    .map(img => ({
                                        src: img.src,
                                        alt: img.alt || '',
                                        width: img.width,
                                        height: img.height
                                    }));

                                // 提取链接
                                const links = Array.from(document.querySelectorAll('a[href]'))
                                    .map(a => ({
                                        href: a.href,
                                        text: a.innerText.trim() || a.title || '无文本'
                                    }));

                                return {
                                    title,
                                    url,
                                    content,
                                    images,
                                    links
                                };
                            }

                            return extractPageContent();
                        }
                    }, (results) => {
                        if (chrome.runtime.lastError) {
                            handleExtractResponse({
                                success: false,
                                error: chrome.runtime.lastError.message
                            });
                            return;
                        }

                        if (!results || !results[0] || !results[0].result) {
                            handleExtractResponse({
                                success: false,
                                error: '内容提取失败'
                            });
                            return;
                        }

                        // 处理提取结果
                        handleExtractResponse({
                            success: true,
                            result: {
                                content: results[0].result,
                                timestamp: new Date().toISOString()
                            }
                        });

                        // 显示成功提示
                        showSuccess('内容提取成功！');
                    });
                });
            } catch (error) {
                handleExtractResponse({
                    success: false,
                    error: error.message
                });
            }
        }
    } catch (error) {
        // 恢复按钮状态
        extractBtn.disabled = false;
        extractBtn.innerHTML = '<span class="icon">📋</span><span class="label">提取当前页面内容</span>';

        showError(error.message);
    }
}

// 处理内容提取响应
function handleExtractResponse(response) {
    // 恢复按钮状态
    extractBtn.disabled = false;
    extractBtn.innerHTML = '<span class="icon">📋</span><span class="label">提取当前页面内容</span>';

    if (!response || !response.success) {
        showError(response?.error || '内容提取失败');
        return;
    }

    // 显示提取结果
    currentResult = {
        type: 'content',
        content: response.result.content
    };

    // 创建内容预览
    const content = response.result.content;
    let previewHtml = '';

    // 标题
    previewHtml += `<h4>${content.title}</h4>`;
    previewHtml += `<p><small>${content.url}</small></p>`;

    // 内容摘要
    if (content.content) {
        const summary = content.content.substring(0, 200) + (content.content.length > 200 ? '...' : '');
        previewHtml += `<p>${summary}</p>`;
    }

    // 图片预览
    if (content.images && content.images.length > 0) {
        previewHtml += `<p><strong>图片数量:</strong> ${content.images.length}</p>`;
        if (content.images.length > 0) {
            previewHtml += `<img src="${content.images[0].src}" alt="${content.images[0].alt || ''}" style="max-height: 100px;">`;
        }
    }

    // 链接数量
    if (content.links) {
        previewHtml += `<p><strong>链接数量:</strong> ${content.links.length}</p>`;
    }

    // 添加复制按钮
    previewHtml += `<div class="preview-footer">
        <button id="copyContentBtn" class="btn small">
            <span class="icon">📋</span>
            <span class="label">复制到剪贴板</span>
        </button>
    </div>`;

    // 显示预览
    previewContent.innerHTML = previewHtml;
    resultPreview.style.display = 'block';

    // 添加复制按钮事件监听
    document.getElementById('copyContentBtn').addEventListener('click', () => {
        copyContentToClipboard(content);
    });
}

// 保存结果
function saveResult() {
    if (!currentResult) return;

    try {
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-');

        if (currentResult.type === 'screenshot') {
            // 保存截图，使用chrome.downloads.download API
            const filename = `screenshot_${timestamp}.png`;

            // 将dataUrl转换为Blob
            fetch(currentResult.dataUrl)
                .then(res => res.blob())
                .then(blob => {
                    const url = URL.createObjectURL(blob);

                    // 使用chrome.downloads.download API，设置saveAs为true让用户选择保存位置
                    chrome.downloads.download({
                        url: url,
                        filename: filename,
                        saveAs: true
                    }, (downloadId) => {
                        if (chrome.runtime.lastError) {
                            showError(`保存失败: ${chrome.runtime.lastError.message}`);
                        } else {
                            showSuccess(`截图已保存为 ${filename}！`);
                        }
                        // 释放URL对象
                        setTimeout(() => URL.revokeObjectURL(url), 100);
                    });
                })
                .catch(error => {
                    showError(`保存失败: ${error.message}`);
                });
        } else if (currentResult.type === 'content') {
            // 保存内容为JSON
            const filename = `content_${timestamp}.json`;
            const blob = new Blob([JSON.stringify(currentResult.content, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);

            // 使用chrome.downloads.download API，设置saveAs为true让用户选择保存位置
            chrome.downloads.download({
                url: url,
                filename: filename,
                saveAs: true
            }, (downloadId) => {
                if (chrome.runtime.lastError) {
                    showError(`保存失败: ${chrome.runtime.lastError.message}`);
                } else {
                    showSuccess(`内容已保存为 ${filename}！`);
                }
                // 释放URL对象
                setTimeout(() => URL.revokeObjectURL(url), 100);
            });
        }
    } catch (error) {
        showError(`保存失败: ${error.message}`);
    }
}

// 显示错误消息
function showError(message) {
    previewContent.innerHTML = `<div class="error-message">${message}</div>`;
    resultPreview.style.display = 'block';
}

// 显示成功提示
function showSuccess(message) {
    const successHtml = `<div class="success-message">${message}</div>`;

    // 如果预览已显示，添加到预览内容顶部
    if (resultPreview.style.display === 'block') {
        previewContent.innerHTML = successHtml + previewContent.innerHTML;
    } else {
        // 否则显示独立的成功提示
        previewContent.innerHTML = successHtml;
        resultPreview.style.display = 'block';

        // 3秒后自动关闭
        setTimeout(() => {
            if (previewContent.querySelector('.success-message') === previewContent.firstChild) {
                resultPreview.style.display = 'none';
            }
        }, 3000);
    }
}

// 检查连接状态
async function checkConnectionStatus() {
    return new Promise((resolve) => {
        console.log("checkConnectionStatus Promise checking...")
        chrome.runtime.sendMessage({ action: 'getStatus' }, (response) => {
            if (response && response.success && response.status && response.status.status === 'connected') {
                console.log("checkConnectionStatus Promise checking...true")
                resolve(true);
            } else {
                console.log("checkConnectionStatus Promise checking...false")
                resolve(false);
            }
        });
    });
}

// 复制内容到剪贴板
function copyContentToClipboard(content) {
    try {
        // 创建要复制的文本
        let copyText = `标题: ${content.title}\n`;
        copyText += `URL: ${content.url}\n\n`;

        if (content.content) {
            copyText += `内容:\n${content.content}\n\n`;
        }

        if (content.images && content.images.length > 0) {
            copyText += `图片 (${content.images.length}):\n`;
            content.images.slice(0, 5).forEach((img, index) => {
                copyText += `${index + 1}. ${img.src}\n`;
            });
            if (content.images.length > 5) {
                copyText += `... 以及 ${content.images.length - 5} 张更多图片\n`;
            }
            copyText += '\n';
        }

        if (content.links && content.links.length > 0) {
            copyText += `链接 (${content.links.length}):\n`;
            content.links.slice(0, 5).forEach((link, index) => {
                copyText += `${index + 1}. ${link.text}: ${link.href}\n`;
            });
            if (content.links.length > 5) {
                copyText += `... 以及 ${content.links.length - 5} 个更多链接\n`;
            }
        }

        // 复制到剪贴板
        navigator.clipboard.writeText(copyText)
            .then(() => {
                showSuccess('内容已复制到剪贴板！');
            })
            .catch((err) => {
                showError('复制失败: ' + err.message);
            });
    } catch (error) {
        showError('复制失败: ' + error.message);
    }
}

// 初始化弹出窗口
document.addEventListener('DOMContentLoaded', initPopup);

// 监听来自后台的消息
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.action === 'status_update' || message.action === 'connectionStatusChanged') {
        updateConnectionStatus();
    }
    return true;
});

// 根据连接状态更新结果预览区域
function updateResultPreviewForConnection(isConnected) {
    if (isConnected) {
        // 连接成功时，显示连接成功的消息
        previewContent.innerHTML = `<div class="success-message">服务器连接成功！现在可以进行页面截图和内容提取操作。</div>`;
        resultPreview.style.display = 'block';

        // 3秒后自动关闭成功提示
        setTimeout(() => {
            // 只有当预览内容仍然是连接成功消息时才关闭
            if (previewContent.querySelector('.success-message') &&
                previewContent.querySelector('.success-message').textContent.includes('服务器连接成功')) {
                resultPreview.style.display = 'none';
            }
        }, 3000);
    } else {
        // 未连接时，如果当前显示的是连接成功消息，则隐藏结果预览
        if (previewContent.querySelector('.success-message') &&
            previewContent.querySelector('.success-message').textContent.includes('服务器连接成功')) {
            resultPreview.style.display = 'none';
        }
        // 如果有其他内容显示，则保持不变
    }
}

// 添加CSS样式
function addStyles() {
    const style = document.createElement('style');
    style.textContent = `
        .success-message {
            background-color: #d4edda;
            color: #155724;
            padding: 10px;
            border-radius: 4px;
            margin-bottom: 10px;
        }
        
        .preview-footer {
            margin-top: 15px;
            display: flex;
            justify-content: flex-start;
        }
    `;
    document.head.appendChild(style);
}

// 在初始化时添加样式
addStyles();