package streamtest

import (
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/renv"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rflowv2"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
)

// MatchAny returns a matcher that accepts any event.
func MatchAny[T any]() func(T) bool {
	return func(T) bool { return true }
}

// --- Environment helpers ---

// ExpectEnv adds an expectation for environment events.
func (v *Verifier) ExpectEnv(
	stream eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(renv.EnvironmentEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[renv.EnvironmentEvent]()
	}
	Expect(v, "Environment", stream, eventType, count,
		func(e renv.EnvironmentEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectEnvInsert is a shorthand for ExpectEnv with Insert type and Exactly(1).
func (v *Verifier) ExpectEnvInsert(
	stream eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent],
	matcher func(renv.EnvironmentEvent) bool,
) *Verifier {
	return v.ExpectEnv(stream, Insert, Exactly(1), matcher)
}

// ExpectEnvVar adds an expectation for environment variable events.
func (v *Verifier) ExpectEnvVar(
	stream eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(renv.EnvironmentVariableEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[renv.EnvironmentVariableEvent]()
	}
	Expect(v, "EnvironmentVariable", stream, eventType, count,
		func(e renv.EnvironmentVariableEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectEnvVarInsert is a shorthand for ExpectEnvVar with Insert type.
func (v *Verifier) ExpectEnvVarInsert(
	stream eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent],
	count CountConstraint,
	matcher func(renv.EnvironmentVariableEvent) bool,
) *Verifier {
	return v.ExpectEnvVar(stream, Insert, count, matcher)
}

// ExpectEnvVarUpdate is a shorthand for ExpectEnvVar with Update type.
func (v *Verifier) ExpectEnvVarUpdate(
	stream eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent],
	count CountConstraint,
	matcher func(renv.EnvironmentVariableEvent) bool,
) *Verifier {
	return v.ExpectEnvVar(stream, Update, count, matcher)
}

// --- HTTP helpers ---

// ExpectHttp adds an expectation for HTTP events.
func (v *Verifier) ExpectHttp(
	stream eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rhttp.HttpEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rhttp.HttpEvent]()
	}
	Expect(v, "Http", stream, eventType, count,
		func(e rhttp.HttpEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectHttpInsert is a shorthand for ExpectHttp with Insert type.
func (v *Verifier) ExpectHttpInsert(
	stream eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent],
	count CountConstraint,
	matcher func(rhttp.HttpEvent) bool,
) *Verifier {
	return v.ExpectHttp(stream, Insert, count, matcher)
}

// ExpectHttpHeader adds an expectation for HTTP header events.
func (v *Verifier) ExpectHttpHeader(
	stream eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rhttp.HttpHeaderEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rhttp.HttpHeaderEvent]()
	}
	Expect(v, "HttpHeader", stream, eventType, count,
		func(e rhttp.HttpHeaderEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectHttpSearchParam adds an expectation for HTTP search param events.
func (v *Verifier) ExpectHttpSearchParam(
	stream eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rhttp.HttpSearchParamEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rhttp.HttpSearchParamEvent]()
	}
	Expect(v, "HttpSearchParam", stream, eventType, count,
		func(e rhttp.HttpSearchParamEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectHttpBodyForm adds an expectation for HTTP body form events.
func (v *Verifier) ExpectHttpBodyForm(
	stream eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rhttp.HttpBodyFormEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rhttp.HttpBodyFormEvent]()
	}
	Expect(v, "HttpBodyForm", stream, eventType, count,
		func(e rhttp.HttpBodyFormEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectHttpBodyUrlEncoded adds an expectation for HTTP body URL-encoded events.
func (v *Verifier) ExpectHttpBodyUrlEncoded(
	stream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rhttp.HttpBodyUrlEncodedEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rhttp.HttpBodyUrlEncodedEvent]()
	}
	Expect(v, "HttpBodyUrlEncoded", stream, eventType, count,
		func(e rhttp.HttpBodyUrlEncodedEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectHttpBodyRaw adds an expectation for HTTP body raw events.
func (v *Verifier) ExpectHttpBodyRaw(
	stream eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rhttp.HttpBodyRawEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rhttp.HttpBodyRawEvent]()
	}
	Expect(v, "HttpBodyRaw", stream, eventType, count,
		func(e rhttp.HttpBodyRawEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectHttpAssert adds an expectation for HTTP assert events.
func (v *Verifier) ExpectHttpAssert(
	stream eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rhttp.HttpAssertEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rhttp.HttpAssertEvent]()
	}
	Expect(v, "HttpAssert", stream, eventType, count,
		func(e rhttp.HttpAssertEvent) string { return e.Type },
		matcher,
	)
	return v
}

// --- Flow helpers ---

// ExpectFlow adds an expectation for flow events.
func (v *Verifier) ExpectFlow(
	stream eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rflowv2.FlowEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rflowv2.FlowEvent]()
	}
	Expect(v, "Flow", stream, eventType, count,
		func(e rflowv2.FlowEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectFlowInsert is a shorthand for ExpectFlow with Insert type and Exactly(1).
func (v *Verifier) ExpectFlowInsert(
	stream eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent],
	matcher func(rflowv2.FlowEvent) bool,
) *Verifier {
	return v.ExpectFlow(stream, Insert, Exactly(1), matcher)
}

// ExpectNode adds an expectation for node events.
func (v *Verifier) ExpectNode(
	stream eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rflowv2.NodeEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rflowv2.NodeEvent]()
	}
	Expect(v, "Node", stream, eventType, count,
		func(e rflowv2.NodeEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectNodeInsert is a shorthand for ExpectNode with Insert type.
func (v *Verifier) ExpectNodeInsert(
	stream eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent],
	count CountConstraint,
	matcher func(rflowv2.NodeEvent) bool,
) *Verifier {
	return v.ExpectNode(stream, Insert, count, matcher)
}

// ExpectEdge adds an expectation for edge events.
func (v *Verifier) ExpectEdge(
	stream eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rflowv2.EdgeEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rflowv2.EdgeEvent]()
	}
	Expect(v, "Edge", stream, eventType, count,
		func(e rflowv2.EdgeEvent) string { return e.Type },
		matcher,
	)
	return v
}

// --- File helpers ---

// ExpectFile adds an expectation for file events.
func (v *Verifier) ExpectFile(
	stream eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rfile.FileEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rfile.FileEvent]()
	}
	// Note: File events use "create" instead of "insert"
	typeStr := string(eventType)
	if eventType == Insert {
		typeStr = "create"
	}
	Expect(v, "File", stream, EventType(typeStr), count,
		func(e rfile.FileEvent) string { return e.Type },
		matcher,
	)
	return v
}

// --- Credential helpers ---

// ExpectCredential adds an expectation for credential events.
func (v *Verifier) ExpectCredential(
	stream eventstream.SyncStreamer[rcredential.CredentialTopic, rcredential.CredentialEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rcredential.CredentialEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rcredential.CredentialEvent]()
	}
	Expect(v, "Credential", stream, eventType, count,
		func(e rcredential.CredentialEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectCredentialInsert is a shorthand for ExpectCredential with Insert type.
func (v *Verifier) ExpectCredentialInsert(
	stream eventstream.SyncStreamer[rcredential.CredentialTopic, rcredential.CredentialEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialEvent) bool,
) *Verifier {
	return v.ExpectCredential(stream, Insert, count, matcher)
}

// ExpectCredentialUpdate is a shorthand for ExpectCredential with Update type.
func (v *Verifier) ExpectCredentialUpdate(
	stream eventstream.SyncStreamer[rcredential.CredentialTopic, rcredential.CredentialEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialEvent) bool,
) *Verifier {
	return v.ExpectCredential(stream, Update, count, matcher)
}

// ExpectCredentialDelete is a shorthand for ExpectCredential with Delete type.
func (v *Verifier) ExpectCredentialDelete(
	stream eventstream.SyncStreamer[rcredential.CredentialTopic, rcredential.CredentialEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialEvent) bool,
) *Verifier {
	return v.ExpectCredential(stream, Delete, count, matcher)
}

// ExpectCredentialOpenAi adds an expectation for credential OpenAI events.
func (v *Verifier) ExpectCredentialOpenAi(
	stream eventstream.SyncStreamer[rcredential.CredentialOpenAiTopic, rcredential.CredentialOpenAiEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rcredential.CredentialOpenAiEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rcredential.CredentialOpenAiEvent]()
	}
	Expect(v, "CredentialOpenAi", stream, eventType, count,
		func(e rcredential.CredentialOpenAiEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectCredentialOpenAiInsert is a shorthand for ExpectCredentialOpenAi with Insert type.
func (v *Verifier) ExpectCredentialOpenAiInsert(
	stream eventstream.SyncStreamer[rcredential.CredentialOpenAiTopic, rcredential.CredentialOpenAiEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialOpenAiEvent) bool,
) *Verifier {
	return v.ExpectCredentialOpenAi(stream, Insert, count, matcher)
}

// ExpectCredentialOpenAiUpdate is a shorthand for ExpectCredentialOpenAi with Update type.
func (v *Verifier) ExpectCredentialOpenAiUpdate(
	stream eventstream.SyncStreamer[rcredential.CredentialOpenAiTopic, rcredential.CredentialOpenAiEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialOpenAiEvent) bool,
) *Verifier {
	return v.ExpectCredentialOpenAi(stream, Update, count, matcher)
}

// ExpectCredentialOpenAiDelete is a shorthand for ExpectCredentialOpenAi with Delete type.
func (v *Verifier) ExpectCredentialOpenAiDelete(
	stream eventstream.SyncStreamer[rcredential.CredentialOpenAiTopic, rcredential.CredentialOpenAiEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialOpenAiEvent) bool,
) *Verifier {
	return v.ExpectCredentialOpenAi(stream, Delete, count, matcher)
}

// ExpectCredentialGemini adds an expectation for credential Gemini events.
func (v *Verifier) ExpectCredentialGemini(
	stream eventstream.SyncStreamer[rcredential.CredentialGeminiTopic, rcredential.CredentialGeminiEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rcredential.CredentialGeminiEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rcredential.CredentialGeminiEvent]()
	}
	Expect(v, "CredentialGemini", stream, eventType, count,
		func(e rcredential.CredentialGeminiEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectCredentialGeminiInsert is a shorthand for ExpectCredentialGemini with Insert type.
func (v *Verifier) ExpectCredentialGeminiInsert(
	stream eventstream.SyncStreamer[rcredential.CredentialGeminiTopic, rcredential.CredentialGeminiEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialGeminiEvent) bool,
) *Verifier {
	return v.ExpectCredentialGemini(stream, Insert, count, matcher)
}

// ExpectCredentialGeminiUpdate is a shorthand for ExpectCredentialGemini with Update type.
func (v *Verifier) ExpectCredentialGeminiUpdate(
	stream eventstream.SyncStreamer[rcredential.CredentialGeminiTopic, rcredential.CredentialGeminiEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialGeminiEvent) bool,
) *Verifier {
	return v.ExpectCredentialGemini(stream, Update, count, matcher)
}

// ExpectCredentialGeminiDelete is a shorthand for ExpectCredentialGemini with Delete type.
func (v *Verifier) ExpectCredentialGeminiDelete(
	stream eventstream.SyncStreamer[rcredential.CredentialGeminiTopic, rcredential.CredentialGeminiEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialGeminiEvent) bool,
) *Verifier {
	return v.ExpectCredentialGemini(stream, Delete, count, matcher)
}

// ExpectCredentialAnthropic adds an expectation for credential Anthropic events.
func (v *Verifier) ExpectCredentialAnthropic(
	stream eventstream.SyncStreamer[rcredential.CredentialAnthropicTopic, rcredential.CredentialAnthropicEvent],
	eventType EventType,
	count CountConstraint,
	matcher func(rcredential.CredentialAnthropicEvent) bool,
) *Verifier {
	if matcher == nil {
		matcher = MatchAny[rcredential.CredentialAnthropicEvent]()
	}
	Expect(v, "CredentialAnthropic", stream, eventType, count,
		func(e rcredential.CredentialAnthropicEvent) string { return e.Type },
		matcher,
	)
	return v
}

// ExpectCredentialAnthropicInsert is a shorthand for ExpectCredentialAnthropic with Insert type.
func (v *Verifier) ExpectCredentialAnthropicInsert(
	stream eventstream.SyncStreamer[rcredential.CredentialAnthropicTopic, rcredential.CredentialAnthropicEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialAnthropicEvent) bool,
) *Verifier {
	return v.ExpectCredentialAnthropic(stream, Insert, count, matcher)
}

// ExpectCredentialAnthropicUpdate is a shorthand for ExpectCredentialAnthropic with Update type.
func (v *Verifier) ExpectCredentialAnthropicUpdate(
	stream eventstream.SyncStreamer[rcredential.CredentialAnthropicTopic, rcredential.CredentialAnthropicEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialAnthropicEvent) bool,
) *Verifier {
	return v.ExpectCredentialAnthropic(stream, Update, count, matcher)
}

// ExpectCredentialAnthropicDelete is a shorthand for ExpectCredentialAnthropic with Delete type.
func (v *Verifier) ExpectCredentialAnthropicDelete(
	stream eventstream.SyncStreamer[rcredential.CredentialAnthropicTopic, rcredential.CredentialAnthropicEvent],
	count CountConstraint,
	matcher func(rcredential.CredentialAnthropicEvent) bool,
) *Verifier {
	return v.ExpectCredentialAnthropic(stream, Delete, count, matcher)
}
