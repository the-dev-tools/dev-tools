package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestInvokedName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		arg0      string
		name      string
		stresseur bool
	}{
		{"stresseur", "stresseur", true},
		{"/usr/local/bin/stresseur", "stresseur", true},
		{"./stresseur", "stresseur", true},
		{"STRESSEUR.EXE", "stresseur", true},
		{`C:\bin\stresseur.exe`, "stresseur", true},
		{`C:\Users\me\bin\Stresseur.Exe`, "stresseur", true},
		{"devtools", "devtools", false},
		{"/usr/local/bin/devtools", "devtools", false},
		{"devtoolscli", "devtoolscli", false},
		{"/home/runner/work/_temp/devtools/bin/devtoolscli", "devtoolscli", false},
		{`C:\bin\devtools.exe`, "devtools", false},
		{"devtools-cli-1.1.1-linux-x64", "devtools-cli-1.1.1-linux-x64", false},
		{"dist/cli", "cli", false},
		{"stresseur-old", "stresseur-old", false},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := invokedName(tc.arg0); got != tc.name {
			t.Errorf("invokedName(%q) = %q, want %q", tc.arg0, got, tc.name)
		}
		if got := invokedAsStresseur(tc.arg0); got != tc.stresseur {
			t.Errorf("invokedAsStresseur(%q) = %v, want %v", tc.arg0, got, tc.stresseur)
		}
	}
}

func TestVersionLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		arg0 string
		want string
	}{
		// Legacy names: byte-identical to the pre-rename output.
		{"devtools", "DevToolsCLI v1.2.3\n"},
		{"/usr/local/bin/devtoolscli", "DevToolsCLI v1.2.3\n"},
		{`C:\bin\devtools.exe`, "DevToolsCLI v1.2.3\n"},
		{"stresseur", "Stresseur CLI v1.2.3 (formerly DevTools CLI)\n"},
		{"/usr/local/bin/stresseur", "Stresseur CLI v1.2.3 (formerly DevTools CLI)\n"},
	}
	for _, tc := range cases {
		if got := versionLine(tc.arg0, "v1.2.3"); got != tc.want {
			t.Errorf("versionLine(%q) = %q, want %q", tc.arg0, got, tc.want)
		}
	}
}

// TestVersionCommandStdout runs the real `version` command under both names and
// checks what lands on stdout. Not parallel: it swaps os.Args and os.Stdout.
func TestVersionCommandStdout(t *testing.T) { //nolint:paralleltest // mutates os.Args/os.Stdout
	cases := map[string]string{
		"devtools":    "DevToolsCLI " + version + "\n",
		"devtoolscli": "DevToolsCLI " + version + "\n",
		"stresseur":   "Stresseur CLI " + version + " (formerly DevTools CLI)\n",
	}
	origArgs, origStdout := os.Args, os.Stdout
	t.Cleanup(func() { os.Args, os.Stdout = origArgs, origStdout })

	for arg0, want := range cases {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Args = []string{arg0, "version"}
		os.Stdout = w
		versionCmd.Run(versionCmd, nil)
		_ = w.Close()
		os.Stdout = origStdout
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != want {
			t.Errorf("%s version printed %q, want %q", arg0, got, want)
		}
	}
}

func TestRenameNoticeEnabled(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		env  map[string]string
		want bool
	}{
		{"devtools", []string{"devtools", "flow", "run"}, nil, true},
		{"devtoolscli", []string{"/tmp/devtools/bin/devtoolscli", "version"}, nil, true},
		{"windows legacy exe", []string{`C:\bin\devtools.exe`}, nil, true},
		{"stresseur never", []string{"stresseur", "version"}, nil, false},
		{"stresseur.exe never", []string{`C:\bin\stresseur.exe`}, nil, false},
		{"empty args", []string{}, nil, false},
		{"tab completion", []string{"devtools", "__complete", "fl"}, nil, false},
		{"tab completion no desc", []string{"devtools", "__completeNoDesc", "fl"}, nil, false},
		{"completion script still notices", []string{"devtools", "completion", "bash"}, nil, true},
		{"silenced new var", []string{"devtools"}, map[string]string{"STRESSEUR_NO_RENAME_NOTICE": "1"}, false},
		{"silenced old var", []string{"devtools"}, map[string]string{"DEVTOOLS_NO_RENAME_NOTICE": "true"}, false},
		{"silenced any value", []string{"devtools"}, map[string]string{"STRESSEUR_NO_RENAME_NOTICE": "yes"}, false},
		{"0 does not silence", []string{"devtools"}, map[string]string{"STRESSEUR_NO_RENAME_NOTICE": "0"}, true},
		{"false does not silence", []string{"devtools"}, map[string]string{"DEVTOOLS_NO_RENAME_NOTICE": "FALSE"}, true},
		{"empty does not silence", []string{"devtools"}, map[string]string{"STRESSEUR_NO_RENAME_NOTICE": ""}, true},
		{"new var off, old var on", []string{"devtools"}, map[string]string{"STRESSEUR_NO_RENAME_NOTICE": "0", "DEVTOOLS_NO_RENAME_NOTICE": "1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(k string) (string, bool) { v, ok := tc.env[k]; return v, ok }
			if got := renameNoticeEnabled(tc.args, lookup); got != tc.want {
				t.Errorf("renameNoticeEnabled(%q, %v) = %v, want %v", tc.args, tc.env, got, tc.want)
			}
		})
	}
}
