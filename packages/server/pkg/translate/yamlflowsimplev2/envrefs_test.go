package yamlflowsimplev2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const envRefsYAML = `
workspace_name: W
environments:
  - name: local
    variables:
      base: '{{ #env:APP_URL }}'
requests:
  - name: Login
    url: '{{base}}/login'
    headers:
      Authorization: 'Bearer {{#env:TOKEN}}'
flows:
  - name: Main
    steps:
      - request:
          name: A
          use_request: Login
          # {{ #env:ONLY_IN_A_COMMENT }}
          body:
            type: json
            json:
              password: '{{ #env:PASSWORD }} and {{ #env:PASSWORD }}'
      - run_sub_flow:
          name: Sub
          depends_on: A
          flow: Helper
  - name: Helper
    steps:
      - request: { name: H, url: '{{ #env:HELPER_URL }}/x' }
  - name: Other
    steps:
      - request: { name: O, url: '{{ #env:OTHER_URL }}/x' }
`

func TestEnvReferences(t *testing.T) {
	got, err := EnvReferences([]byte(envRefsYAML), []string{"Main"})
	require.NoError(t, err)
	require.Equal(t, []string{"APP_URL", "HELPER_URL", "PASSWORD", "TOKEN"}, got)

	all, err := EnvReferences([]byte(envRefsYAML), nil)
	require.NoError(t, err)
	require.Equal(t, []string{"APP_URL", "HELPER_URL", "OTHER_URL", "PASSWORD", "TOKEN"}, all)

	none, err := EnvReferences([]byte("workspace_name: W\nflows: []\n"), nil)
	require.NoError(t, err)
	require.Empty(t, none)
}
