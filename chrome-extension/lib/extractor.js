/**
 * 内容提取功能模块
 * 负责智能识别和提取网页主要内容区域
 */

(function (global) {
    class ContentExtractor {
        constructor() {
            this.extractionRules = [];
            this.loadCustomRules();
        }

        /**
         * 从存储中加载自定义提取规则
         */
        loadCustomRules() {
            chrome.storage.sync.get(['extractionRules'], (result) => {
                if (result.extractionRules && Array.isArray(result.extractionRules)) {
                    this.extractionRules = result.extractionRules;
                }
            });
        }

        /**
         * 提取当前页面的主要内容
         * @param {number} tabId 标签页ID
         * @param {Object} options 提取选项
         * @returns {Promise<Object>} 提取的内容
         */
        async extractContent(tabId, options = {}) {
            try {
                // 获取页面URL，用于匹配自定义规则
                const tab = await this.getTabInfo(tabId);
                const url = tab.url;

                // 检查是否有匹配的自定义规则
                const customRule = this.findMatchingRule(url);

                if (customRule) {
                    // 使用自定义规则提取内容
                    return await this.extractWithCustomRule(tabId, customRule);
                } else {
                    // 使用通用算法提取内容
                    return await this.extractWithGeneralAlgorithm(tabId, options);
                }
            } catch (error) {
                console.error('内容提取失败:', error);
                throw error;
            }
        }

        /**
         * 获取标签页信息
         * @param {number} tabId 标签页ID
         * @returns {Promise<Object>} 标签页信息
         */
        async getTabInfo(tabId) {
            return new Promise((resolve, reject) => {
                chrome.tabs.get(tabId, (tab) => {
                    if (chrome.runtime.lastError) {
                        reject(new Error(chrome.runtime.lastError.message));
                    } else {
                        resolve(tab);
                    }
                });
            });
        }

        /**
         * 查找匹配URL的自定义规则
         * @param {string} url 页面URL
         * @returns {Object|null} 匹配的规则或null
         */
        findMatchingRule(url) {
            return this.extractionRules.find(rule => {
                try {
                    const pattern = new RegExp(rule.urlPattern);
                    return pattern.test(url);
                } catch (e) {
                    console.error('规则匹配错误:', e);
                    return false;
                }
            });
        }

        /**
         * 使用自定义规则提取内容
         * @param {number} tabId 标签页ID
         * @param {Object} rule 自定义规则
         * @returns {Promise<Object>} 提取的内容
         */
        async extractWithCustomRule(tabId, rule) {
            return new Promise((resolve, reject) => {
                chrome.scripting.executeScript({
                    target: { tabId },
                    function: (selectors) => {
                        const result = {};

                        // 遍历选择器并提取内容
                        for (const key in selectors) {
                            const selector = selectors[key];
                            const elements = document.querySelectorAll(selector);

                            if (elements.length > 0) {
                                if (elements.length === 1) {
                                    // 单个元素
                                    result[key] = elements[0].innerText.trim();
                                } else {
                                    // 多个元素
                                    result[key] = Array.from(elements).map(el => el.innerText.trim());
                                }
                            }
                        }

                        // 添加页面元数据
                        result.title = document.title;
                        result.url = window.location.href;

                        return result;
                    },
                    args: [rule.selectors]
                }, (results) => {
                    if (chrome.runtime.lastError) {
                        reject(new Error(chrome.runtime.lastError.message));
                    } else if (!results || !results[0]) {
                        reject(new Error('内容提取失败'));
                    } else {
                        resolve(results[0].result);
                    }
                });
            });
        }

        /**
         * 使用通用算法提取内容
         * @param {number} tabId 标签页ID
         * @param {Object} options 提取选项
         * @returns {Promise<Object>} 提取的内容
         */
        /**
         * 确保 defuddle bundle 已注入到目标页面
         * @param {number} tabId 标签页ID
         * @returns {Promise<void>}
         */
        async ensureDefuddleInjected(tabId) {
            const isInjected = await new Promise((resolve) => {
                chrome.scripting.executeScript({
                    target: { tabId },
                    func: () => typeof window.__defuddleExtract === 'function'
                }, (results) => {
                    resolve(results && results[0] && results[0].result === true);
                });
            });

            if (!isInjected) {
                await new Promise((resolve, reject) => {
                    chrome.scripting.executeScript({
                        target: { tabId },
                        files: ['lib/defuddle.bundle.js']
                    }, () => {
                        if (chrome.runtime.lastError) {
                            reject(new Error('注入 defuddle 失败: ' + chrome.runtime.lastError.message));
                        } else {
                            resolve();
                        }
                    });
                });
            }
        }

        /**
         * 使用通用算法提取内容（基于 defuddle）
         * @param {number} tabId 标签页ID
         * @param {Object} options 提取选项
         * @returns {Promise<Object>} 提取的内容
         */
        async extractWithGeneralAlgorithm(tabId, options) {
            // 确保 defuddle 已注入
            await this.ensureDefuddleInjected(tabId);

            return new Promise((resolve, reject) => {
                chrome.scripting.executeScript({
                    target: { tabId },
                    func: (opts) => {
                        // 调用 defuddle 进行提取，直接返回 Markdown
                        const result = window.__defuddleExtract({
                            markdown: true,
                            url: window.location.href,
                            standardize: true,
                            removeExactSelectors: true,
                            removePartialSelectors: true,
                            removeHiddenElements: true,
                            removeLowScoring: true,
                            removeSmallImages: true,
                            ...opts
                        });
                        return result;
                    },
                    args: [options]
                }, (results) => {
                    if (chrome.runtime.lastError) {
                        reject(new Error(chrome.runtime.lastError.message));
                    } else if (!results || !results[0]) {
                        reject(new Error('内容提取失败'));
                    } else {
                        const d = results[0].result;
                        // 适配为原有格式，content 现在是 Markdown
                        // title/url 由调用方（background.js）从 tab 信息补充
                        resolve({
                            title: d.title || '',
                            description: d.description || '',
                            url: d.domain ? ('https://' + d.domain) : '',
                            content: d.content || '',           // Markdown
                            markdown: d.content || '',          // 冗余字段，明确标识为 Markdown
                            author: d.author || '',
                            published: d.published || '',
                            site: d.site || '',
                            language: d.language || '',
                            wordCount: d.wordCount || 0,
                            image: d.image || '',
                            favicon: d.favicon || '',
                            domain: d.domain || '',
                            // 向后兼容字段
                            html: '',
                            images: [],
                            links: [],
                            textLength: d.wordCount || 0
                        });
                    }
                });
            });
        }

        /**
         * 添加自定义提取规则
         * @param {Object} rule 规则对象
         * @returns {Promise<boolean>} 是否成功
         */
        async addCustomRule(rule) {
            // 验证规则格式
            if (!rule.name || !rule.urlPattern || !rule.selectors) {
                throw new Error('规则格式无效');
            }

            // 检查URL模式是否有效
            try {
                new RegExp(rule.urlPattern);
            } catch (e) {
                throw new Error('URL模式无效: ' + e.message);
            }

            // 添加规则
            this.extractionRules.push(rule);

            // 保存到存储
            return new Promise((resolve, reject) => {
                chrome.storage.sync.set({ extractionRules: this.extractionRules }, () => {
                    if (chrome.runtime.lastError) {
                        reject(new Error(chrome.runtime.lastError.message));
                    } else {
                        resolve(true);
                    }
                });
            });
        }

        /**
         * 删除自定义提取规则
         * @param {string} ruleName 规则名称
         * @returns {Promise<boolean>} 是否成功
         */
        async removeCustomRule(ruleName) {
            const initialLength = this.extractionRules.length;
            this.extractionRules = this.extractionRules.filter(rule => rule.name !== ruleName);

            // 如果没有变化，说明规则不存在
            if (initialLength === this.extractionRules.length) {
                return false;
            }

            // 保存到存储
            return new Promise((resolve, reject) => {
                chrome.storage.sync.set({ extractionRules: this.extractionRules }, () => {
                    if (chrome.runtime.lastError) {
                        reject(new Error(chrome.runtime.lastError.message));
                    } else {
                        resolve(true);
                    }
                });
            });
        }

        /**
         * 获取所有自定义提取规则
         * @returns {Array} 规则列表
         */
        getCustomRules() {
            return [...this.extractionRules];
        }
    }

    // 创建单例实例并导出到全局作用域
    global.contentExtractor = new ContentExtractor();
    global.ContentExtractor = ContentExtractor;
})(self);
