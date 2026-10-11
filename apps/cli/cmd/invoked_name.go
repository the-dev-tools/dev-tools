package cmd

import (
	"strings"
)

// stresseurName is the new user-facing command name. The binary is also
// installed as "devtools" (install.sh) and "devtoolscli" (GitHub Action);
// every name other than this one is treated as the legacy name and behaves
// exactly as the CLI did before the rename.
const stresseurName = "stresseur"

// invokedName returns the command name the user typed: the last path element
// of argv[0] (split on both "/" and "\" so Windows paths work on any OS),
// lowercased, with a trailing ".exe" removed.
func invokedName(arg0 string) string {
	name := arg0
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToLower(name)
	return strings.TrimSuffix(name, ".exe")
}

// invokedAsStresseur reports whether the CLI was started under the new
// "stresseur" name. This is the single switch for name-dependent behaviour.
func invokedAsStresseur(arg0 string) bool {
	return invokedName(arg0) == stresseurName
}

// renameNotice is printed to stderr when the CLI runs under a legacy name.
const renameNotice = "devtools is now stresseur. The devtools command keeps working; switch to `stresseur` when you can."

// Either variable silences renameNotice. The new name is listed first.
var renameNoticeSilencers = []string{"STRESSEUR_NO_RENAME_NOTICE", "DEVTOOLS_NO_RENAME_NOTICE"}

// renameNoticeEnabled reports whether Execute should print renameNotice.
// It is shown only under a legacy name (devtools, devtoolscli, ...), never
// for "stresseur", and never for cobra's hidden __complete/__completeNoDesc
// commands that shells call during tab completion. Setting
// STRESSEUR_NO_RENAME_NOTICE or DEVTOOLS_NO_RENAME_NOTICE to any non-empty
// value other than "0" or "false" (case-insensitive) silences it.
//
// The planned later phase (~Jan 2027, legacy names print a message and exit 0)
// is one added line after the notice is printed in Execute: os.Exit(0).
func renameNoticeEnabled(args []string, lookup func(string) (string, bool)) bool {
	if len(args) == 0 || invokedAsStresseur(args[0]) {
		return false
	}
	if len(args) > 1 && strings.HasPrefix(args[1], "__complete") {
		return false
	}
	for _, key := range renameNoticeSilencers {
		if v, ok := lookup(key); ok && envTruthy(v) {
			return false
		}
	}
	return true
}

// envTruthy treats any non-empty value except "0" and "false" as set.
func envTruthy(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}
