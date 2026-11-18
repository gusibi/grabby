package nativeui

import "github.com/egoist/mygo/ui"

func mustIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

func mustFillIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">` + paths + `</svg>`))
}

var (
	iconToday    = mustIcon(`<rect x="2.5" y="3.5" width="15" height="13" rx="2.5"/><path d="M2.5 8h15M6.5 2.5v2M13.5 2.5v2"/>`)
	iconLibrary  = mustIcon(`<rect x="2.5" y="3" width="6" height="14" rx="1.5"/><rect x="10.5" y="3" width="7" height="6" rx="1.5"/><rect x="10.5" y="11" width="7" height="6" rx="1.5"/>`)
	iconRSS      = mustIcon(`<circle cx="5" cy="15" r="1.4"/><path d="M3.5 3.5v5a8 8 0 0 0 8 8h2M3.5 3.5a13 13 0 0 1 13 13"/>`)
	iconNews     = mustIcon(`<rect x="2.5" y="4" width="15" height="12.5" rx="2"/><path d="M5 7.5h5v4H5zM12 7.5h3M12 10h3M5 13.5h10"/>`)
	iconAI       = mustFillIcon(`<path d="M10 2.2l1.5 4.1 4.3.3-3.3 2.7 1.1 4.2L10 11.2l-3.6 2.3 1.1-4.2L4.2 6.6l4.3-.3z"/>`)
	iconDaily    = mustIcon(`<rect x="3" y="2.5" width="14" height="15" rx="2"/><path d="M6.5 6.5h7M6.5 9.5h7M6.5 12.5h4"/>`)
	iconDevices  = mustIcon(`<rect x="2" y="3.5" width="16" height="11" rx="2"/><path d="M7 17h6M10 14.5V17"/>`)
	iconAPI      = mustIcon(`<path d="M7 5 3 10l4 5M13 5l4 5-4 5M11.5 3.5l-3 13"/>`)
	iconSettings = mustIcon(`<circle cx="10" cy="10" r="3"/><path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.4 4.4l1.4 1.4M14.2 14.2l1.4 1.4M15.6 4.4l-1.4 1.4M5.8 14.2l-1.4 1.4"/>`)
	iconSearch   = mustIcon(`<circle cx="8.5" cy="8.5" r="5"/><path d="M12.2 12.2 17 17"/>`)
	iconStar     = mustFillIcon(`<path d="M10 2.5l2.3 4.7 5.2.8-3.8 3.7.9 5.2L10 14.4l-4.6 2.5.9-5.2L2.5 8l5.2-.8z"/>`)
	iconStarLine = mustIcon(`<path d="M10 2.8l2.2 4.5 5 .7-3.6 3.5.8 5L10 14.2l-4.4 2.3.8-5L2.8 8l5-.7z"/>`)
	iconExternal = mustIcon(`<path d="M8 4H4.5A1.5 1.5 0 0 0 3 5.5v10A1.5 1.5 0 0 0 4.5 17h10a1.5 1.5 0 0 0 1.5-1.5V12"/><path d="M11.5 3H17v5.5M17 3l-7 7"/>`)
	iconClose    = mustIcon(`<path d="M5 5l10 10M15 5 5 15"/>`)
	iconPlus     = mustIcon(`<path d="M10 4v12M4 10h12"/>`)
	iconRefresh  = mustIcon(`<path d="M16.2 10a6.2 6.2 0 1 1-1.9-4.5"/><path d="M16.5 3v4.2h-4.2"/>`)
	iconChevronL = mustIcon(`<path d="M12.5 4.5 7 10l5.5 5.5"/>`)
	iconChevronR = mustIcon(`<path d="M7.5 4.5 13 10l-5.5 5.5"/>`)
	iconGrid     = mustIcon(`<rect x="3" y="3" width="6" height="6" rx="1"/><rect x="11" y="3" width="6" height="6" rx="1"/><rect x="3" y="11" width="6" height="6" rx="1"/><rect x="11" y="11" width="6" height="6" rx="1"/>`)
	iconList     = mustIcon(`<path d="M3.5 5h13M3.5 10h13M3.5 15h13"/>`)
	iconCheck    = mustIcon(`<path d="M4 10.5 8 14.5 16 5.5"/>`)
	iconInbox    = mustIcon(`<path d="M3 11.5V6a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v5.5"/><path d="M3 11.5h4l1.2 2h3.6l1.2-2h4V15a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	iconPlay     = mustFillIcon(`<path d="M6.5 4.2v11.6L16 10z"/>`)
	iconPause    = mustFillIcon(`<rect x="5.5" y="4" width="3.2" height="12" rx="1"/><rect x="11.3" y="4" width="3.2" height="12" rx="1"/>`)
	iconTrash    = mustIcon(`<path d="M3.5 5.5h13M8 3h4M5.5 5.5l.7 10a1.5 1.5 0 0 0 1.5 1.4h4.6a1.5 1.5 0 0 0 1.5-1.4l.7-10"/>`)
	iconEdit     = mustIcon(`<path d="M13.5 3.5l3 3L7 16H4v-3z"/>`)
	iconBolt     = mustFillIcon(`<path d="M11 2 4 11h5l-1 7 7-9h-5z"/>`)
	iconCopy     = mustIcon(`<rect x="6.5" y="6.5" width="10" height="10" rx="2"/><path d="M13.5 6.5v-2a2 2 0 0 0-2-2h-7a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h2"/>`)
)
