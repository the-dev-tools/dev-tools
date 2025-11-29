package harv2_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/translate/harv2"
)

func TestReorganizeNodePositions_Sequential(t *testing.T) {
	// Scenario: Start -> Login -> Profile (Sequential)
	// Expectation:
	// Start: Level 0, Y=0
	// Login: Level 1, Y=300
	// Profile: Level 2, Y=600

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
	nodes := make(map[string]mnnode.MNode)
	for _, n := range result.Nodes {
		nodes[n.Name] = n
	}

	start, ok := nodes["Start"]
	require.True(t, ok)
	
	// Names are generated, let's assume "POST Login" and "GET Profile" or similar if name generation is consistent
	// Or check the nodes list order if names are tricky.
	// harv2 generates names like "request_1", "request_2" for Node Name field in processEntries.
	// Wait, processEntries sets Name to "request_%d".
	// "Start" is explicit.
	
	req1, ok := nodes["request_1"]
	require.True(t, ok, "request_1 not found")
	req2, ok := nodes["request_2"]
	require.True(t, ok, "request_2 not found")

	// Verify Y Positions (Depth)
	assert.Equal(t, 0.0, start.PositionY, "Start should be at Y=0")
	assert.Equal(t, 300.0, req1.PositionY, "Request 1 should be at Y=300")
	assert.Equal(t, 600.0, req2.PositionY, "Request 2 should be at Y=600")

	// Verify X Positions (should be centered, so 0 if single node per level)
	assert.Equal(t, 0.0, start.PositionX)
	assert.Equal(t, 0.0, req1.PositionX)
	assert.Equal(t, 0.0, req2.PositionX)
}

func TestReorganizeNodePositions_Parallel(t *testing.T) {
	// Scenario: Start -> [Req A, Req B] (Parallel/Branching)
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
			Request: harv2.Request{Method: "GET", URL: "https://api.com/a"},
		},
		{
			StartedDateTime: time.Now().Add(10 * time.Second), // Far apart
			Request: harv2.Request{Method: "GET", URL: "https://api.com/b"},
		},
	}

	har := &harv2.HAR{Log: harv2.Log{Entries: entries}}
	workspaceID := idwrap.NewNow()
	depFinder := depfinder.NewDepFinder()

	result, err := harv2.ConvertHARWithDepFinder(har, workspaceID, &depFinder)
	require.NoError(t, err)

	nodes := make(map[string]mnnode.MNode)
	for _, n := range result.Nodes {
		nodes[n.Name] = n
	}

	start := nodes["Start"]
	req1 := nodes["request_1"]
	req2 := nodes["request_2"]

	// Expectation:
	// Level 0: Start
	// Level 1: Req1, Req2
	
	assert.Equal(t, 0.0, start.PositionY)
	assert.Equal(t, 300.0, req1.PositionY)
	assert.Equal(t, 300.0, req2.PositionY) // Same level

	// X Positions should differ
	assert.NotEqual(t, req1.PositionX, req2.PositionX)
	
	// Center alignment calculation:
	// 2 nodes, spacing 400.
	// Total width = (2-1)*400 = 400.
	// StartX = 0 - 400/2 = -200.
	// Node 0 X = -200 + 0 = -200
	// Node 1 X = -200 + 400 = 200
	
	// We don't enforce specific order in the map iteration in layout (it uses slice from map), 
	// so we just check they are -200 and 200.
	assert.True(t, (req1.PositionX == -200 && req2.PositionX == 200) || (req1.PositionX == 200 && req2.PositionX == -200))
}
