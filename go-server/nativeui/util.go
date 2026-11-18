package nativeui

import "github.com/egoist/mygo"

func openExternal(url string) error {
	return mygo.Shell.OpenExternal(url)
}

func openPath(path string) error {
	if path == "" {
		return nil
	}
	return mygo.Shell.OpenPath(path)
}

func clipboardWrite(s string) {
	mygo.Clipboard.WriteText(s)
}
