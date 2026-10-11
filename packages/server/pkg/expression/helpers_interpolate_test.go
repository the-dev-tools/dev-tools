package expression

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The data helpers a request can use in its URL, headers and body. Documented in the
// yamlflow README ("Data helpers"); keep the two in sync.
func TestInterpolateDataHelpers(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{})
	cases := map[string]*regexp.Regexp{
		"{{ uuid() }}":                     regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$`),
		`{{ uuid("v7") }}`:                 regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$`),
		"{{ ulid() }}":                     regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`),
		"{{ faker.email() }}":              regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[a-z]+$`),
		"{{ faker.username() }}":           regexp.MustCompile(`^\S+$`),
		"{{ faker.name() }}":               regexp.MustCompile(`\S`),
		"{{ faker.password() }}":           regexp.MustCompile(`^\S+$`),
		"{{ faker.randomInt(5, 9) }}":      regexp.MustCompile(`^[5-9]$`),
		"{{ now() }}":                      regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}(Z|[+-]\d\d:\d\d)$`),
		"{{ now().Unix() }}":               regexp.MustCompile(`^\d{10}$`),
		"{{ now().UnixMilli() }}":          regexp.MustCompile(`^\d{13}$`),
		`{{ now().Format("2006-01-02") }}`: regexp.MustCompile(`^\d{4}-\d\d-\d\d$`),
		"user-{{ ulid() }}@example.test":   regexp.MustCompile(`^user-[0-9A-Z]{26}@example\.test$`),
	}
	for in, want := range cases {
		out, err := env.Interpolate(in)
		require.NoError(t, err, in)
		require.Regexp(t, want, out, in)
	}

	out, err := env.Interpolate("{{ now().Unix() }}")
	require.NoError(t, err)
	sec, err := strconv.ParseInt(out, 10, 64)
	require.NoError(t, err)
	require.InDelta(t, time.Now().Unix(), sec, 5)

	a, _ := env.Interpolate("{{ ulid() }}")
	b, _ := env.Interpolate("{{ ulid() }}")
	require.NotEqual(t, a, b, "each call generates a new value")
}
