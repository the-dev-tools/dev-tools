package resolver_test

import (
	"testing"

	"github.com/DevToolsGit/devtools-nodes/pkg/resolver"
)

func TestResolver_ResolveNodeFunc(t *testing.T) {
	arrFuncs := []string{resolver.ApiCallRest, resolver.IFStatusCode, resolver.CommunicationEmail}

	for _, funcName := range arrFuncs {
		_, err := resolver.ResolveNodeFunc(funcName)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}
