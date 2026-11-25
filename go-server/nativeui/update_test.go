package nativeui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUpdateSectionStates(t *testing.T) {
	cases := []struct {
		name  string
		state updateState
		want  string
	}{
		{"idle", updateState{}, "关于与更新"},
		{"downloading", updateState{busy: true, phase: "downloading", message: "正在下载 v0.9.0… 42%", progress: 0.42}, "正在下载 v0.9.0… 42%"},
		{"up-to-date", updateState{message: "已是最新版本（v0.8.0）", upToDate: true}, "已是最新版本"},
		{"error", updateState{err: "检查更新失败: boom"}, "检查更新失败"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := fixtureApp()
			a.view = "settings"
			a.update = tc.state
			tester := ui.NewTester(a.View, 1280, 840)
			tester.Frame()
			if !tester.HasText(tc.want) {
				t.Fatalf("settings view does not show %q", tc.want)
			}
		})
	}
}
