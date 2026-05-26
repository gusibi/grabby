import Defuddle from 'defuddle/full';

// 暴露到全局，供 chrome.scripting.executeScript 的 func 使用
window.__Defuddle = Defuddle;
window.__defuddleExtract = function(options) {
    const instance = new Defuddle(document, options);
    return instance.parse();
};
