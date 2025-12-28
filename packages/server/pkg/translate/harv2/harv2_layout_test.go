package harv2_test

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/harv2"

	"github.com/stretchr/testify/require"
)

func TestReorganizeNodePositions_Sequential(t *testing.T) {
	// Scenario: Start -> Login -> Profile (Sequential, horizontal flow)
	// Expectation (horizontal layout):
	// Start: Level 0, X=0
	// Login: Level 1, X=300
	// Profile: Level 2, X=600

	entries := []harv2.Entry{
		{
			StartedDateTime: time.Now(),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.com/login",
			},
			Response: harv2.Response{
				Content: harv2.Content{
					Text: `{"token": "abc"}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(1 * time.Second),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.com/profile",
				Headers: []harv2.Header{
					{Name: "Auth", Value: "abc"},
				},
			},
		},
	}

	har := &harv2.HAR{Log: harv2.Log{Entries: entries}}
	workspaceID := idwrap.NewNow()
	depFinder := depfinder.NewDepFinder()

	result, err := harv2.ConvertHARWithDepFinder(har, workspaceID, &depFinder)
	require.NoError(t, err)

	// Map nodes by name for easy lookup
	nodes := make(map[string]mflow.Node)
	for _, n := range result.Nodes {
		nodes[n.Name] = n
	}

	start, ok := nodes["Start"]
	require.True(t, ok)

	// harv2 generates names like "http_1", "http_2" for Node Name field in processEntries.
	// "Start" is explicit.

	req1, ok := nodes["http_1"]
	require.True(t, ok, "http_1 not found")
	req2, ok := nodes["http_2"]
	require.True(t, ok, "http_2 not found")

	// Verify X Positions (Depth - horizontal flow)
	require.Equal(t, 0.0, start.PositionX, "Start should be at X=0")
	require.Equal(t, 300.0, req1.PositionX, "Request 1 should be at X=300")
	require.Equal(t, 600.0, req2.PositionX, "Request 2 should be at X=600")

	// Verify Y Positions (should be centered, so 0 if single node per level)
	require.Equal(t, 0.0, start.PositionY)
	require.Equal(t, 0.0, req1.PositionY)
	require.Equal(t, 0.0, req2.PositionY)
}

func TestReorganizeNodePositions_Parallel(t *testing.T) {
	// Scenario: Start -> [Req A, Req B] (Parallel/Branching, horizontal flow)
	// If Req A and Req B both depend only on Start (or same parent), they should be on same level.
	// Note: HAR import usually linearizes by timestamp or mutation chain.
	// To force parallel, we need them to have NO dependency on each other, and close timestamps might trigger sequential edge.
	// But `processEntries` connects orphans to Start.
	// If we have 2 GET requests with NO data dependency and NO timestamp/mutation link (e.g. far apart but no dependency?),
	// actually current logic links based on "Previous Node" for timestamp sequencing.
	// So hard to get parallel nodes unless we break the timestamp/sequential logic or use specific dependency graph.

	// However, the positioning algorithm supports parallel nodes (nodes at same level).
	// Let's try to construct a scenario where A and B both depend on Start but not each other.
	// Since `processEntries` links `previousNode -> currentNode` by default for most cases...
	// Actually, looking at `processEntries`:
	// 1. Data Dependency
	// 2. Timestamp Sequencing (if diff <= 50ms)
	// 3. Mutation Chain (if Mutation)
	// 4. Sequential Ordering (if DELETE)
	// 5. Rooting (Connect orphans to Start)

	// If we have GET A (t=0) and GET B (t=10s).
	// Time diff > 50ms. No timestamp edge.
	// Not mutation.
	// No data dep.
	// So A connects to Start (Orphan).
	// B connects to Start (Orphan).
	// So they should be parallel at Level 1.

	entries := []harv2.Entry{
		{
			StartedDateTime: time.Now(),
			Request:         harv2.Request{Method: "GET", URL: "https://api.com/a"},
		},
		{
			StartedDateTime: time.Now().Add(10 * time.Second), // Far apart
			Request:         harv2.Request{Method: "GET", URL: "https://api.com/b"},
		},
	}

	har := &harv2.HAR{Log: harv2.Log{Entries: entries}}
	workspaceID := idwrap.NewNow()
	depFinder := depfinder.NewDepFinder()

	result, err := harv2.ConvertHARWithDepFinder(har, workspaceID, &depFinder)
	require.NoError(t, err)

	nodes := make(map[string]mflow.Node)
	for _, n := range result.Nodes {
		nodes[n.Name] = n
	}

	start := nodes["Start"]
	req1 := nodes["http_1"]
	req2 := nodes["http_2"]

	// Expectation (horizontal layout):
	// Level 0: Start (X=0)
	// Level 1: Req1, Req2 (X=300, stacked vertically on Y axis)

	require.Equal(t, 0.0, start.PositionX)
	require.Equal(t, 300.0, req1.PositionX)
	require.Equal(t, 300.0, req2.PositionX) // Same X level (horizontal)

	// Y Positions should differ (parallel nodes stacked vertically)
	require.NotEqual(t, req1.PositionY, req2.PositionY)

	// Center alignment calculation (vertical stacking):
	// 2 nodes, spacing 150.
	// Total height = (2-1)*150 = 150.
	// StartY = 0 - 150/2 = -75.
	// Node 0 Y = -75 + 0 = -75
	// Node 1 Y = -75 + 150 = 75

	// We don't enforce specific order in the map iteration in layout (it uses slice from map),
	// so we just check they are -75 and 75.
	require.True(t, (req1.PositionY == -75 && req2.PositionY == 75) || (req1.PositionY == 75 && req2.PositionY == -75))
}
