package yamlflowsimplev2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func convertFlowYAML(t *testing.T, yamlData string) error {
	t.Helper()
	_, err := ConvertSimplifiedYAML([]byte(yamlData), GetDefaultOptions(idwrap.NewNow()))
	return err
}

// The AI judge reads Ask's answer but doesn't depend on Ask, so it would run at flow start
// with the template unfilled and the flow would pass.
func TestReferenceWithoutDependsOnIsRejectedForAIStep(t *testing.T) {
	err := convertFlowYAML(t, `
workspace_name: W
credentials:
  - name: llm
    type: openai
    token: x
flows:
  - name: F
    steps:
      - request:
          name: Ask
          method: POST
          url: https://api.example.com/ask
      - ai_provider:
          name: Model
          credential: llm
          model: gpt-4o
      - ai:
          name: Judge
          provider: Model
          prompt: "Grade this answer: {{ Ask.response.body.answer }}"
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Judge")
	require.Contains(t, err.Error(), "Ask")
	require.Contains(t, err.Error(), "depends_on")
}

func TestReferenceWithoutDependsOnIsRejectedForRequestStep(t *testing.T) {
	err := convertFlowYAML(t, `
workspace_name: W
flows:
  - name: F
    steps:
      - manual_start:
          name: Start
      - request:
          name: Login
          depends_on: Start
          method: POST
          url: https://api.example.com/login
      - request:
          name: Me
          depends_on: Start
          method: GET
          url: https://api.example.com/me
          headers:
            Authorization: "Bearer {{ Login.response.body.token }}"
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), `step 'Me' uses 'Login.response.body.token'`)
	require.Contains(t, err.Error(), "depends_on: Login")
}

func TestReferenceInAssertionWithoutDependsOnIsRejected(t *testing.T) {
	err := convertFlowYAML(t, `
workspace_name: W
flows:
  - name: F
    steps:
      - request:
          name: Create
          method: POST
          url: https://api.example.com/items
      - request:
          name: Get
          method: GET
          url: https://api.example.com/items/1
          assertions:
            - response.body.id == Create.response.body.id
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "step 'Get' uses 'Create.response.body.id'")
}

func TestTransitiveDependencyAndLoopReferencesAreAccepted(t *testing.T) {
	err := convertFlowYAML(t, `
workspace_name: W
flows:
  - name: F
    variables:
      - name: base
        value: https://api.example.com
    steps:
      - manual_start:
          name: Start
      - request:
          name: Login
          depends_on: Start
          method: POST
          url: "{{ base }}/login"
      - request:
          name: List
          depends_on: Login
          method: GET
          url: "{{ base }}/items"
          headers:
            Authorization: "Bearer {{ Login.response.body.token }}"
      - for_each:
          name: EachItem
          depends_on: List
          items: List.response.body.items
          loop: Fetch
      - request:
          name: Fetch
          method: GET
          url: "{{ base }}/items/{{ EachItem.item.id }}"
          headers:
            Authorization: "Bearer {{ Login.response.body.token }}"
          assertions:
            - response.status == 200
            - "{{ EachItem.item.id }} > 0"
      - if:
          name: Check
          depends_on: List
          condition: len(List.response.body.items) > 0
          then: Done
      - js:
          name: Done
          code: |
            export default function(ctx) { return ctx.Login; }
`)
	require.NoError(t, err)
}

// A step that never runs (explicit start, no depends_on) can't run with an unfilled
// template, so it's left alone.
func TestDisconnectedStepIsNotChecked(t *testing.T) {
	err := convertFlowYAML(t, `
workspace_name: W
flows:
  - name: F
    steps:
      - manual_start:
          name: Start
      - request:
          name: Login
          depends_on: Start
          method: POST
          url: https://api.example.com/login
      - request:
          name: Draft
          method: GET
          url: "https://api.example.com/{{ Login.response.body.id }}"
`)
	require.NoError(t, err)
}

// break_condition is evaluated after each iteration, so it may read the loop body.
func TestBreakConditionMayReadLoopBody(t *testing.T) {
	err := convertFlowYAML(t, `
workspace_name: W
flows:
  - name: F
    steps:
      - for:
          name: BreakLoop
          iter_count: 10
          loop: FetchUser
          break_condition: FetchUser.response.body.username == "Bret"
      - request:
          name: FetchUser
          method: GET
          url: https://api.example.com/users/1
      - request:
          name: Other
          method: GET
          url: https://api.example.com/other
`)
	require.NoError(t, err)

	err = convertFlowYAML(t, `
workspace_name: W
flows:
  - name: F
    steps:
      - for:
          name: BreakLoop
          iter_count: 10
          loop: FetchUser
          break_condition: Other.response.status == 200
      - request:
          name: FetchUser
          method: GET
          url: https://api.example.com/users/1
      - request:
          name: Other
          method: GET
          url: https://api.example.com/other
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "step 'BreakLoop' uses 'Other.response.status'")
}

func TestReferenceToRequestTemplateBodyIsChecked(t *testing.T) {
	err := convertFlowYAML(t, `
workspace_name: W
requests:
  - name: me
    method: GET
    url: https://api.example.com/me
    headers:
      Authorization: "Bearer {{ Login.response.body.token }}"
flows:
  - name: F
    steps:
      - request:
          name: Login
          method: POST
          url: https://api.example.com/login
      - request:
          name: Me
          use_request: me
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "step 'Me' uses 'Login.response.body.token'")
}
