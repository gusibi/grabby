/**
 * 弹出窗口脚本
 * 负责连接状态、截图/提取操作，以及结果预览的渲染。
 */

import {
    escapeHtml,
    escapeAttr,
    stripFrontMatter,
    countCharacters,
    shortUrl,
    renderMarkdown
} from './markdown.js';

// ---------- DOM ----------
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');
const serverInfo = document.getElementById('serverInfo');
const connectBtn = document.getElementById('connectBtn');
const disconnectBtn = document.getElementById('disconnectBtn');
const captureBtn = document.getElementById('captureBtn');
const extractBtn = document.getElementById('extractBtn');
const resultBody = document.getElementById('previewContent');
const resultMeta = document.getElementById('resultMeta');
const saveResultBtn = document.getElementById('saveResultBtn');
const closePreviewBtn = document.getElementById('closePreviewBtn');
const openOptionsBtn = document.getElementById('openOptionsBtn');
const toastEl = document.getElementById('toast');

const EMPTY_STATE_HTML = `
  <div class="empty-state">
    <span class="empty-mark" aria-hidden="true">“</span>
    <p class="empty-title">还没有结果</p>
    <p class="empty-hint">截图首屏，或提取整页正文转成 Markdown。</p>
  </div>`;

const CAPTURE_LABELS = { title: '截图', sub: '当前首屏', busyTitle: '截图中…', busySub: '请稍候' };
const EXTRACT_LABELS = { title: '提取内容', sub: '全文转 Markdown', busyTitle: '提取中…', busySub: '请稍候' };

let currentResult = null;
let toastTimer = null;

// ---------- 初始化 ----------

function initPopup() {
    showEmptyState();
    updateConnectionStatus();
    loadConfig();
    setupEventListeners();

    chrome.runtime.onMessage.addListener((message) => {
        if (message.action === 'connectionStatusChanged' || message.action === 'status_update') {
            updateConnectionStatus();
        }
    });
}

function loadConfig() {
    chrome.storage.sync.get(['serverUrl'], (result) => {
        if (result.serverUrl) renderServerUrl(result.serverUrl);
    });
}

function setupEventListeners() {
    connectBtn.addEventListener('click', () => {
        chrome.runtime.sendMessage({ action: 'connect' }, (response) => {
            if (response && response.success) {
                updateConnectionStatus();
                showToast('服务器已连接', 'success');
            } else {
                showError((response && response.error) || '连接服务器失败，请检查服务器配置');
                updateConnectionStatus();
            }
        });
    });

    disconnectBtn.addEventListener('click', () => {
        chrome.runtime.sendMessage({ action: 'disconnect' }, (response) => {
            if (response && response.success) updateConnectionStatus();
        });
    });

    captureBtn.addEventListener('click', captureCurrentTab);
    extractBtn.addEventListener('click', extractCurrentTab);
    saveResultBtn.addEventListener('click', saveResult);

    document.getElementById('copyResultBtn').addEventListener('click', copyResult);
    closePreviewBtn.addEventListener('click', showEmptyState);

    openOptionsBtn.addEventListener('click', () => chrome.runtime.openOptionsPage());
    document.getElementById('openLogsBtn').addEventListener('click', () => {
        chrome.tabs.create({ url: chrome.runtime.getURL('logs/logs.html') });
    });

    // 预览里的链接在弹窗内点击会顶掉整个弹窗，改为新标签页打开。
    resultBody.addEventListener('click', (event) => {
        const link = event.target.closest('a[href]');
        if (!link) return;
        event.preventDefault();
        const href = link.getAttribute('href') || '';
        if (/^https?:/i.test(href) || /^mailto:/i.test(href)) {
            chrome.tabs.create({ url: href });
        }
    });

    window.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') showEmptyState();
    });
}

// ---------- 连接状态 ----------

function updateConnectionUI(status) {
    if (!status) return;

    statusDot.className = 'status-dot ' + status.status;

    switch (status.status) {
        case 'connected':
            statusText.textContent = '已连接';
            connectBtn.disabled = true;
            disconnectBtn.disabled = false;
            break;
        case 'connecting':
            statusText.textContent = '连接中…';
            connectBtn.disabled = true;
            disconnectBtn.disabled = !status.autoReconnect;
            break;
        case 'error':
            statusText.textContent = '连接错误';
            connectBtn.disabled = false;
            disconnectBtn.disabled = !status.autoReconnect;
            break;
        default:
            statusText.textContent = '未连接';
            connectBtn.disabled = false;
            disconnectBtn.disabled = !status.autoReconnect;
    }

    if (status.serverUrl) renderServerUrl(status.serverUrl);
}

function updateConnectionStatus() {
    chrome.runtime.sendMessage({ action: 'getStatus' }, (response) => {
        if (!response || !response.success) return;
        updateConnectionUI(response.status);
    });
}

function renderServerUrl(url) {
    serverInfo.textContent = url || '未配置';
    serverInfo.title = url || '';
}

// ---------- 操作按钮状态 ----------

function setActionBusy(button, busy, labels) {
    button.classList.toggle('is-busy', busy);
    button.disabled = busy;
    const title = button.querySelector('.action-title');
    const sub = button.querySelector('.action-sub');
    if (title) title.textContent = busy ? labels.busyTitle : labels.title;
    if (sub) sub.textContent = busy ? labels.busySub : labels.sub;
}

function setResultActions(enabled) {
    saveResultBtn.disabled = !enabled;
    document.getElementById('copyResultBtn').disabled = !enabled;
    closePreviewBtn.disabled = !enabled;
}

// ---------- 截图 ----------

function captureCurrentTab() {
    setActionBusy(captureBtn, true, CAPTURE_LABELS);

    chrome.runtime.sendMessage({
        action: 'captureTab',
        // 只截当前可见的首屏，不滚动、不拼接整页。
        options: { fullPage: false }
    }, (response) => {
        setActionBusy(captureBtn, false, CAPTURE_LABELS);
        if (!response) {
            showError((chrome.runtime.lastError && chrome.runtime.lastError.message) || '后台无响应');
            return;
        }
        handleCaptureResponse(response);
    });
}

function handleCaptureResponse(response) {
    if (!response || !response.success) {
        showError((response && response.error) || '截图失败');
        return;
    }

    const result = response.result || {};
    const dataUrl = result.dataUrl || result.imageData;
    if (!dataUrl) {
        showError('截图结果为空');
        return;
    }

    currentResult = { type: 'screenshot', dataUrl };

    resultBody.classList.remove('is-empty');
    resultBody.innerHTML = `
      <div class="shot">
        <img id="shotImage" src="${dataUrl}" alt="页面截图">
        <span class="shot-caption">当前首屏 · 不滚动、不拼接整页</span>
      </div>`;
    resultMeta.textContent = '截图 · 当前首屏';
    setResultActions(true);
    resultBody.scrollTop = 0;

    const image = document.getElementById('shotImage');
    if (image) {
        image.addEventListener('load', () => {
            const kb = Math.max(1, Math.round((dataUrl.length * 0.75) / 1024));
            resultMeta.textContent = `截图 · ${image.naturalWidth}×${image.naturalHeight} · ${kb} KB`;
        }, { once: true });
    }

    showToast('截图完成', 'success');
}

// ---------- 提取 ----------

function extractCurrentTab() {
    setActionBusy(extractBtn, true, EXTRACT_LABELS);

    chrome.runtime.sendMessage({
        action: 'extractContent',
        options: {}
    }, (response) => {
        setActionBusy(extractBtn, false, EXTRACT_LABELS);
        if (!response) {
            showError((chrome.runtime.lastError && chrome.runtime.lastError.message) || '后台无响应');
            return;
        }
        handleExtractResponse(response);
    });
}

function handleExtractResponse(response) {
    if (!response || !response.success) {
        showError((response && response.error) || '内容提取失败');
        return;
    }

    const content = response.result && response.result.content;
    if (!content) {
        showError('提取结果为空');
        return;
    }

    currentResult = { type: 'content', content };

    const markdown = String(content.content || content.markdown || '');
    const body = stripFrontMatter(markdown).trim();
    const title = String(content.title || '未命名页面').trim();
    const url = String(content.url || '').trim();

    resultBody.classList.remove('is-empty');

    if (!body) {
        resultBody.innerHTML = `
          <div class="error-note">没有提取到正文，只有页面元信息。可以刷新页面后重试。</div>`;
        resultMeta.textContent = '未提取到正文';
        setResultActions(true);
        resultBody.scrollTop = 0;
        return;
    }

    const characters = countCharacters(body);
    const imageCount = (body.match(/!\[[^\]]*\]\(/g) || []).length;
    const linkCount = Math.max(0, (body.match(/\[[^\]]+\]\(/g) || []).length - imageCount);

    const stats = [`正文 ${characters.toLocaleString()} 字符`];
    if (imageCount) stats.push(`${imageCount} 张图片`);
    if (linkCount) stats.push(`${linkCount} 个链接`);

    const safeSource = /^https?:\/\//i.test(url) ? url : '';

    resultBody.innerHTML = `
      <article class="doc">
        <h2 class="doc-title">${escapeHtml(title)}</h2>
        ${safeSource
            ? `<a class="doc-source" href="${escapeAttr(safeSource)}" title="${escapeAttr(url)}">${escapeHtml(shortUrl(url))}</a>`
            : ''}
        <div class="doc-stats">${stats.map((s) => `<span class="stat">${escapeHtml(s)}</span>`).join('')}</div>
        <div class="doc-body">${renderMarkdown(body, safeSource)}</div>
      </article>`;

    resultMeta.textContent = '已提取 · Markdown';
    setResultActions(true);
    resultBody.scrollTop = 0;

    showToast('内容提取完成', 'success');
}

// ---------- 预览状态 ----------

function showEmptyState() {
    currentResult = null;
    resultBody.classList.add('is-empty');
    resultBody.innerHTML = EMPTY_STATE_HTML;
    resultMeta.textContent = '等待操作';
    setResultActions(false);
    resultBody.scrollTop = 0;
}

function showToast(message, kind = '') {
    toastEl.textContent = message;
    toastEl.className = `toast ${kind} show`;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
        toastEl.className = `toast ${kind}`;
    }, 3200);
}

function showError(message) {
    showToast(message, 'error');

    // 没有结果可展示时，把错误留在预览区，避免提示一闪而过。
    if (!currentResult) {
        resultBody.classList.remove('is-empty');
        resultBody.innerHTML = `<div class="error-note">${escapeHtml(message)}</div>`;
        resultMeta.textContent = '操作失败';
        setResultActions(false);
        resultBody.scrollTop = 0;
    }
}

// ---------- 保存 / 复制 ----------

function saveResult() {
    if (!currentResult) return;

    try {
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-');

        if (currentResult.type === 'screenshot') {
            const filename = `screenshot_${timestamp}.png`;

            fetch(currentResult.dataUrl)
                .then((res) => res.blob())
                .then((blob) => {
                    const url = URL.createObjectURL(blob);
                    chrome.downloads.download({ url, filename, saveAs: true }, () => {
                        if (chrome.runtime.lastError) {
                            showError(`保存失败: ${chrome.runtime.lastError.message}`);
                        } else {
                            showToast('截图已保存', 'success');
                        }
                        setTimeout(() => URL.revokeObjectURL(url), 100);
                    });
                })
                .catch((error) => showError(`保存失败: ${error.message}`));
            return;
        }

        const markdown = String(currentResult.content.content || currentResult.content.markdown || '');
        if (!markdown) {
            showError('没有可保存的内容');
            return;
        }

        const filename = `grabby-${timestamp}.md`;
        const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' });
        const url = URL.createObjectURL(blob);

        chrome.downloads.download({ url, filename, saveAs: true }, () => {
            if (chrome.runtime.lastError) {
                showError(`保存失败: ${chrome.runtime.lastError.message}`);
            } else {
                showToast('Markdown 已保存', 'success');
            }
            setTimeout(() => URL.revokeObjectURL(url), 100);
        });
    } catch (error) {
        showError(`保存失败: ${error.message}`);
    }
}

function copyResult() {
    if (!currentResult) return;

    if (currentResult.type === 'screenshot') {
        try {
            navigator.clipboard.write([
                new ClipboardItem({
                    'image/png': fetch(currentResult.dataUrl).then((r) => r.blob())
                })
            ]).then(() => {
                showToast('截图已复制', 'success');
            }).catch((err) => showError('复制截图失败: ' + err.message));
        } catch (error) {
            showError('当前浏览器不支持复制图片，请使用保存。');
        }
        return;
    }

    const text = String(currentResult.content.content || currentResult.content.markdown || '');
    if (!text) {
        showError('没有可复制的内容');
        return;
    }

    navigator.clipboard.writeText(text)
        .then(() => showToast('Markdown 已复制', 'success'))
        .catch((err) => showError('复制失败: ' + err.message));
}

// ---------- 启动 ----------

document.addEventListener('DOMContentLoaded', initPopup);
