/**
 * 预览用的轻量 Markdown 渲染。
 * 只支持 Defuddle 产出里常见的语法；所有文本先转义，URL 做协议白名单。
 */

export function stripFrontMatter(markdown) {
    return String(markdown || '').replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/, '');
}

export function countCharacters(text) {
    return String(text || '').replace(/\s+/g, '').length;
}

export function shortUrl(raw) {
    try {
        const url = new URL(raw);
        let value = url.hostname.replace(/^www\./, '') + url.pathname + url.search;
        value = value.replace(/\/$/, '');
        return value.length > 60 ? value.slice(0, 59) + '…' : value;
    } catch (_) {
        return String(raw || '');
    }
}

export function escapeHtml(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, (c) => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    })[c]);
}

export function escapeAttr(value) {
    return escapeHtml(value);
}

export function safeUrl(raw, base, allowDataImage) {
    try {
        const url = new URL(raw, base || undefined);
        if (url.protocol === 'http:' || url.protocol === 'https:' || url.protocol === 'mailto:') {
            return url.href;
        }
        if (allowDataImage && /^data:image\//i.test(raw)) return raw;
        return '';
    } catch (_) {
        return '';
    }
}

export function renderMarkdown(markdown, baseUrl) {
    const lines = String(markdown || '').replace(/\r\n?/g, '\n').split('\n');
    const out = [];
    let paragraph = [];
    let i = 0;

    // 中文软换行不要塞空格，英文行之间保留一个空格
    const joinLines = (linesToJoin) => linesToJoin.reduce((acc, line) => {
        if (!acc) return line;
        const prev = acc[acc.length - 1];
        const next = line[0];
        const isCjk = (ch) => /[\u3000-\u9fff\uf900-\ufaff]/.test(ch || '');
        return acc + (isCjk(prev) && isCjk(next) ? '' : ' ') + line;
    }, '');

    const flushParagraph = () => {
        if (!paragraph.length) return;
        out.push(`<p>${renderInline(joinLines(paragraph), baseUrl)}</p>`);
        paragraph = [];
    };

    while (i < lines.length) {
        const line = lines[i];
        const trimmed = line.trim();

        if (!trimmed) {
            flushParagraph();
            i++;
            continue;
        }

        // 代码块
        const fence = trimmed.match(/^```([\w-]*)\s*$/);
        if (fence) {
            flushParagraph();
            const language = fence[1];
            const buffer = [];
            i++;
            while (i < lines.length && !lines[i].trim().startsWith('```')) {
                buffer.push(lines[i]);
                i++;
            }
            i++;
            const langAttr = language ? ` class="language-${escapeAttr(language)}"` : '';
            out.push(`<pre><code${langAttr}>${escapeHtml(buffer.join('\n'))}</code></pre>`);
            continue;
        }

        // 标题：正文里的 # 降级到 h3 起，避免压过文档标题
        const heading = trimmed.match(/^(#{1,6})\s+(.*)$/);
        if (heading) {
            flushParagraph();
            const level = Math.min(5, Math.max(3, heading[1].length + 1));
            out.push(`<h${level}>${renderInline(heading[2], baseUrl)}</h${level}>`);
            i++;
            continue;
        }

        // 分割线
        if (/^([-*_])\s*(\1\s*){2,}$/.test(trimmed)) {
            flushParagraph();
            out.push('<hr>');
            i++;
            continue;
        }

        // 引用
        if (/^>\s?/.test(trimmed)) {
            flushParagraph();
            const buffer = [];
            while (i < lines.length && /^>\s?/.test(lines[i].trim())) {
                buffer.push(lines[i].trim().replace(/^>\s?/, ''));
                i++;
            }
            out.push(`<blockquote>${renderMarkdown(buffer.join('\n'), baseUrl)}</blockquote>`);
            continue;
        }

        // 列表
        if (/^(?:[-*+]|\d+[.)])\s+/.test(trimmed)) {
            flushParagraph();
            const ordered = /^\d+[.)]\s+/.test(trimmed);
            const items = [];
            while (i < lines.length) {
                const current = lines[i].trim();
                if (!current) {
                    const next = lines[i + 1] ? lines[i + 1].trim() : '';
                    const nextIsItem = /^(?:[-*+]|\d+[.)])\s+/.test(next);
                    const nextOrdered = /^\d+[.)]\s+/.test(next);
                    // 空行后只有同一类型列表的项才算延续
                    if (nextIsItem && nextOrdered === ordered) {
                        i++;
                        continue;
                    }
                    break;
                }
                if (/^\d+[.)]\s+/.test(current) !== ordered) break;
                const item = current.match(/^(?:[-*+]|\d+[.)])\s+(.*)$/);
                if (!item) break;
                items.push(item[1]);
                i++;
            }
            const tag = ordered ? 'ol' : 'ul';
            out.push(`<${tag}>${items.map((item) => `<li>${renderInline(item, baseUrl)}</li>`).join('')}</${tag}>`);
            continue;
        }

        // 表格
        if (trimmed.includes('|') && lines[i + 1] && /^\s*\|?[\s:|-]+\|[\s:|-]*$/.test(lines[i + 1])) {
            flushParagraph();
            const parseRow = (row) => row.replace(/^\s*\|/, '').replace(/\|\s*$/, '').split('|').map((cell) => cell.trim());
            const header = parseRow(trimmed);
            i += 2;
            const rows = [];
            while (i < lines.length && lines[i].trim() && lines[i].includes('|')) {
                rows.push(parseRow(lines[i].trim()));
                i++;
            }
            const head = header.map((cell) => `<th>${renderInline(cell, baseUrl)}</th>`).join('');
            const bodyRows = rows.map((row) => `<tr>${row.map((cell) => `<td>${renderInline(cell, baseUrl)}</td>`).join('')}</tr>`).join('');
            out.push(`<table><thead><tr>${head}</tr></thead><tbody>${bodyRows}</tbody></table>`);
            continue;
        }

        paragraph.push(trimmed);
        i++;
    }

    flushParagraph();
    return out.join('');
}

function renderInline(raw, baseUrl) {
    const stash = [];
    const keep = (html) => `\u0000${stash.push(html) - 1}\u0000`;
    let text = String(raw || '');

    // 行内代码先摘出来，避免里面的 * [ ] 被当成格式
    text = text.replace(/`([^`]+)`/g, (_, code) => keep(`<code>${escapeHtml(code)}</code>`));

    // 图片
    text = text.replace(/!\[([^\]]*)\]\(\s*((?:[^()\s]|\([^()\s]*\))+)(?:\s+"[^"]*")?\s*\)/g, (match, alt, src) => {
        const safe = safeUrl(src, baseUrl, true);
        if (!safe) return match;
        return keep(`<img src="${escapeAttr(safe)}" alt="${escapeAttr(alt)}" loading="lazy" referrerpolicy="no-referrer">`);
    });

    // 链接
    text = text.replace(/\[([^\]]+)\]\(\s*((?:[^()\s]|\([^()\s]*\))+)(?:\s+"[^"]*")?\s*\)/g, (match, label, href) => {
        const safe = safeUrl(href, baseUrl, false);
        if (!safe) return escapeHtml(label);
        return keep(`<a href="${escapeAttr(safe)}" target="_blank" rel="noreferrer noopener">${escapeHtml(label)}</a>`);
    });

    // 普通文本转义后再处理强调
    text = escapeHtml(text);
    text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    text = text.replace(/(?<!\*)\*([^*\n]+)\*(?!\*)/g, '<em>$1</em>');
    text = text.replace(/(?<![_\w])_([^_\n]+)_(?![_\w])/g, '<em>$1</em>');

    // 裸链接
    text = text.replace(/(^|[\s（(])(https?:\/\/[^\s<）)]+)/g, (_, prefix, url) => {
        return `${prefix}${keep(`<a href="${escapeAttr(url)}" target="_blank" rel="noreferrer noopener">${escapeHtml(shortUrl(url))}</a>`)}`;
    });

    return text.replace(/\u0000(\d+)\u0000/g, (_, index) => stash[Number(index)]);
}
