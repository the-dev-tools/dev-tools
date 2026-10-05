package main

import "testing"

func TestResolveMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		env     map[string]string
		value   string
		varName string
	}{
		{"neither set", map[string]string{}, "", EnvDevToolsMode},
		{"only DEVTOOLS_MODE", map[string]string{"DEVTOOLS_MODE": "cli"}, "cli", EnvDevToolsMode},
		{"only STRESSEUR_MODE", map[string]string{"STRESSEUR_MODE": "server"}, "server", EnvStresseurMode},
		{"both set, new wins", map[string]string{"DEVTOOLS_MODE": "server", "STRESSEUR_MODE": "cli"}, "cli", EnvStresseurMode},
		{"empty STRESSEUR_MODE is unset", map[string]string{"DEVTOOLS_MODE": "cli", "STRESSEUR_MODE": ""}, "cli", EnvDevToolsMode},
		{"both empty", map[string]string{"DEVTOOLS_MODE": "", "STRESSEUR_MODE": ""}, "", EnvDevToolsMode},
		{"bogus legacy value reported under legacy name", map[string]string{"DEVTOOLS_MODE": "bogus"}, "bogus", EnvDevToolsMode},
		{"bogus new value reported under new name", map[string]string{"STRESSEUR_MODE": "bogus", "DEVTOOLS_MODE": "cli"}, "bogus", EnvStresseurMode},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(k string) (string, bool) { v, ok := tc.env[k]; return v, ok }
			value, varName := resolveMode(lookup)
			if value != tc.value || varName != tc.varName {
				t.Errorf("resolveMode() = (%q, %q), want (%q, %q)", value, varName, tc.value, tc.varName)
			}
		})
	}
}
