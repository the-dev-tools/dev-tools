package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	homedir "github.com/mitchellh/go-homedir"
)

// Without a config file the CLI says nothing and writes nothing: programs that embed it
// (Stresseur's stress) print its output as their own.
func TestInitConfigWithoutAFileIsSilent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	homedir.DisableCache = true
	t.Cleanup(func() { homedir.DisableCache = false })

	stdout, stderr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = w, w
	initConfig()
	os.Stdout, os.Stderr = stdout, stderr
	_ = w.Close()
	out, _ := io.ReadAll(r)

	if len(out) != 0 {
		t.Fatalf("initConfig printed %q", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".devtools.yaml")); !os.IsNotExist(err) {
		t.Fatalf("initConfig wrote %s (stat err %v)", filepath.Join(home, ".devtools.yaml"), err)
	}
}
