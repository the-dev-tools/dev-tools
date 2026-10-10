package aicheck

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// KeyEnvironment is where judge keys are read from: the dotenv file at dotenvPath (when it
// exists), overridden by the process environment.
func KeyEnvironment(dotenvPath string) map[string]string {
	env := map[string]string{}
	if dotenvPath != "" {
		if b, err := os.ReadFile(filepath.Clean(dotenvPath)); err == nil {
			for k, v := range ParseDotEnv(b) {
				env[k] = v
			}
		}
	}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	return env
}

// ParseDotEnv reads KEY=VALUE lines: `export ` prefixes, # comments and quoted values are
// accepted; anything else is skipped.
func ParseDotEnv(b []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		} else if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		out[k] = v
	}
	return out
}
