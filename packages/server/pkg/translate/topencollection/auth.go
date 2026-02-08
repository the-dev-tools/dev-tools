package topencollection

import (
	"encoding/base64"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// convertAuth converts OpenCollection auth config into HTTP headers or search params.
// Returns additional headers and search params to append.
func convertAuth(auth *OCAuth, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, []mhttp.HTTPSearchParam) {
	if auth == nil {
		return nil, nil
	}

	switch auth.Type {
	case "bearer":
		return []mhttp.HTTPHeader{{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Key:     "Authorization",
			Value:   fmt.Sprintf("Bearer %s", auth.Token),
			Enabled: true,
		}}, nil

	case "basic":
		encoded := base64.StdEncoding.EncodeToString(
			[]byte(fmt.Sprintf("%s:%s", auth.Username, auth.Password)),
		)
		return []mhttp.HTTPHeader{{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Key:     "Authorization",
			Value:   fmt.Sprintf("Basic %s", encoded),
			Enabled: true,
		}}, nil

	case "apikey":
		if auth.Placement == "query" {
			return nil, []mhttp.HTTPSearchParam{{
				ID:      idwrap.NewNow(),
				HttpID:  httpID,
				Key:     auth.Key,
				Value:   auth.Value,
				Enabled: true,
			}}
		}
		// Default placement is header
		return []mhttp.HTTPHeader{{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Key:     auth.Key,
			Value:   auth.Value,
			Enabled: true,
		}}, nil

	case "none", "inherit", "":
		return nil, nil

	default:
		return nil, nil
	}
}
