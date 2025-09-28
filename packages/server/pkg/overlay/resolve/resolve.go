package resolve

import (
	"context"

	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/overlay/merge"
)

// RequestInput bundles the data required to resolve origin and delta request state.
type RequestInput struct {
	BaseExample  mitemapiexample.ItemApiExample
	BaseHeaders  []mexampleheader.Header
	BaseQueries  []mexamplequery.Query
	BaseRawBody  *mbodyraw.ExampleBodyRaw
	BaseFormBody []mbodyform.BodyForm
	BaseURLBody  []mbodyurl.BodyURLEncoded
	BaseAsserts  []massert.Assert

	DeltaExample  *mitemapiexample.ItemApiExample
	DeltaHeaders  []mexampleheader.Header
	DeltaQueries  []mexamplequery.Query
	DeltaRawBody  *mbodyraw.ExampleBodyRaw
	DeltaFormBody []mbodyform.BodyForm
	DeltaURLBody  []mbodyurl.BodyURLEncoded
	DeltaAsserts  []massert.Assert
}

// Request resolves base and delta data into a single view, applying overlay rules
// when a manager is provided. The returned MergeExamplesOutput mirrors the helper
// used by the runtime flow engine, keeping behaviours aligned between execution
// and export paths.
func Request(ctx context.Context, mgr *merge.Manager, input RequestInput, deltaExampleID *idwrap.IDWrap) (request.MergeExamplesOutput, error) {
	baseRaw := valueOrZero(input.BaseRawBody)
	baseForm := cloneForm(input.BaseFormBody)
	baseURL := cloneURL(input.BaseURLBody)
	baseHeaders := cloneHeaders(input.BaseHeaders)
	baseQueries := cloneQueries(input.BaseQueries)
	baseAsserts := cloneAsserts(input.BaseAsserts)

	if deltaExampleID == nil || input.DeltaExample == nil {
		return request.MergeExamplesOutput{
			Merged:              input.BaseExample,
			MergeHeaders:        baseHeaders,
			MergeQueries:        baseQueries,
			MergeRawBody:        baseRaw,
			MergeFormBody:       baseForm,
			MergeUrlEncodedBody: baseURL,
			MergeAsserts:        baseAsserts,
		}, nil
	}

	deltaHeaders := cloneHeaders(input.DeltaHeaders)
	deltaQueries := cloneQueries(input.DeltaQueries)

	if mgr != nil {
		var err error
		deltaHeaders, err = mgr.MergeHeaders(ctx, baseHeaders, deltaHeaders, *deltaExampleID)
		if err != nil {
			return request.MergeExamplesOutput{}, err
		}
		deltaQueries, err = mgr.MergeQueries(ctx, baseQueries, deltaQueries, *deltaExampleID)
		if err != nil {
			return request.MergeExamplesOutput{}, err
		}
	}

	deltaRaw := valueOrZero(input.DeltaRawBody)
	deltaForm := cloneForm(input.DeltaFormBody)
	deltaURL := cloneURL(input.DeltaURLBody)
	deltaAsserts := cloneAsserts(input.DeltaAsserts)

	output := request.MergeExamples(request.MergeExamplesInput{
		Base:                input.BaseExample,
		Delta:               *input.DeltaExample,
		BaseQueries:         baseQueries,
		DeltaQueries:        deltaQueries,
		BaseHeaders:         baseHeaders,
		DeltaHeaders:        deltaHeaders,
		BaseRawBody:         baseRaw,
		DeltaRawBody:        deltaRaw,
		BaseFormBody:        baseForm,
		DeltaFormBody:       deltaForm,
		BaseUrlEncodedBody:  baseURL,
		DeltaUrlEncodedBody: deltaURL,
		BaseAsserts:         baseAsserts,
		DeltaAsserts:        deltaAsserts,
	})

	return output, nil
}

func valueOrZero(body *mbodyraw.ExampleBodyRaw) mbodyraw.ExampleBodyRaw {
	if body == nil {
		return mbodyraw.ExampleBodyRaw{}
	}
	return *body
}

func cloneHeaders(in []mexampleheader.Header) []mexampleheader.Header {
	if len(in) == 0 {
		return nil
	}
	out := make([]mexampleheader.Header, len(in))
	copy(out, in)
	return out
}

func cloneQueries(in []mexamplequery.Query) []mexamplequery.Query {
	if len(in) == 0 {
		return nil
	}
	out := make([]mexamplequery.Query, len(in))
	copy(out, in)
	return out
}

func cloneForm(in []mbodyform.BodyForm) []mbodyform.BodyForm {
	if len(in) == 0 {
		return nil
	}
	out := make([]mbodyform.BodyForm, len(in))
	copy(out, in)
	return out
}

func cloneURL(in []mbodyurl.BodyURLEncoded) []mbodyurl.BodyURLEncoded {
	if len(in) == 0 {
		return nil
	}
	out := make([]mbodyurl.BodyURLEncoded, len(in))
	copy(out, in)
	return out
}

func cloneAsserts(in []massert.Assert) []massert.Assert {
	if len(in) == 0 {
		return nil
	}
	out := make([]massert.Assert, len(in))
	copy(out, in)
	return out
}
