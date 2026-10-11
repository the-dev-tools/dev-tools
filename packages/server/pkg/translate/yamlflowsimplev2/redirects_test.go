package yamlflowsimplev2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

const redirectsYAML = `
workspace_name: Acme
flows:
  - name: Session
    steps:
      - request:
          name: Dashboard
          method: GET
          url: https://app.test/dashboard
          follow_redirects: false
          assertions:
            - response.status == 302
      - request:
          name: Home
          depends_on: Dashboard
          url: https://app.test/
          follow_redirects: true
          assertions:
            - '!(response.url endsWith "/login")'
    cleanup:
      - request:
          name: Logout
          url: https://app.test/logout
          follow_redirects: false
`

func TestFollowRedirectsRoundTrip(t *testing.T) {
	b, err := ConvertSimplifiedYAML([]byte(redirectsYAML), GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err)
	dashboard := httpIDByNode(t, b, nodeIDByName(t, b, "Dashboard"))
	home := httpIDByNode(t, b, nodeIDByName(t, b, "Home"))
	require.Equal(t, []idwrap.IDWrap{dashboard}, b.HTTPNoRedirects)
	all := b.AllHTTPNoRedirects()
	require.Len(t, all, 2, "Dashboard and the Logout cleanup step")
	require.False(t, all[home])

	out, err := MarshalSimplifiedYAML(b)
	require.NoError(t, err)
	text := string(out)
	require.Equal(t, 2, strings.Count(text, "follow_redirects: false"), text)
	require.NotContains(t, text, "follow_redirects: true")

	again, err := ConvertSimplifiedYAML(out, GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err, text)
	require.True(t, again.AllHTTPNoRedirects()[httpIDByNode(t, again, nodeIDByName(t, again, "Dashboard"))])
	require.Len(t, again.AllHTTPNoRedirects(), 2)
}
