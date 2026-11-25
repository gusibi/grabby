//go:build darwin || linux

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelperScriptsAreValidShell(t *testing.T) {
	staged := &Staged{
		Dir:     "/tmp/grabby update 'stage'",
		Path:    "/tmp/Grabby.app",
		Version: "0.9.0",
	}
	scripts := map[string]string{
		"darwin": darwinScript("/Applications/Grabby.app", staged),
		"linux":  linuxScript("/usr/local/bin/grabby", staged),
	}
	for name, body := range scripts {
		path := filepath.Join(t.TempDir(), name+".sh")
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("/bin/sh", "-n", path).CombinedOutput(); err != nil {
			t.Fatalf("%s script is not valid shell: %v\n%s", name, err, out)
		}
	}
}

func TestShellQuoting(t *testing.T) {
	got := shQuote("/Applications/It's Grabby.app")
	want := `'/Applications/It'\''s Grabby.app'`
	if got != want {
		t.Fatalf("shQuote = %s, want %s", got, want)
	}
	if strings.Contains(shQuote("a b"), `\`) {
		t.Fatal("quoted paths must not rely on backslash escapes")
	}
}
