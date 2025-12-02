package rhttp

import (
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mhttpassert"

	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func httpSyncResponseFrom(event HttpEvent) *apiv1.HttpSyncResponse {
	var value *apiv1.HttpSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		name := event.Http.GetName()
		method := event.Http.GetMethod()
		url := event.Http.GetUrl()
		bodyKind := event.Http.GetBodyKind()
		lastRunAt := event.Http.GetLastRunAt()
		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpSyncInsert{
				HttpId:    event.Http.GetHttpId(),
				Name:      name,
				Method:    method,
				Url:       url,
				BodyKind:  bodyKind,
				LastRunAt: lastRunAt,
			},
		}
	case eventTypeUpdate:
		name := event.Http.GetName()
		method := event.Http.GetMethod()
		url := event.Http.GetUrl()
		bodyKind := event.Http.GetBodyKind()
		lastRunAt := event.Http.GetLastRunAt()

		var lastRunAtUnion *apiv1.HttpSyncUpdate_LastRunAtUnion
		if lastRunAt != nil {
			lastRunAtUnion = &apiv1.HttpSyncUpdate_LastRunAtUnion{
				Kind:  apiv1.HttpSyncUpdate_LastRunAtUnion_KIND_VALUE,
				Value: lastRunAt,
			}
		}

		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpSyncUpdate{
				HttpId:    event.Http.GetHttpId(),
				Name:      &name,
				Method:    &method,
				Url:       &url,
				BodyKind:  &bodyKind,
				LastRunAt: lastRunAtUnion,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpSyncDelete{
				HttpId: event.Http.GetHttpId(),
			},
		}
	}

	return &apiv1.HttpSyncResponse{
		Items: []*apiv1.HttpSync{
			{
				Value: value,
			},
		},
	}
}

// httpHeaderSyncResponseFrom converts HttpHeaderEvent to HttpHeaderSync response
func httpHeaderSyncResponseFrom(event HttpHeaderEvent) *apiv1.HttpHeaderSyncResponse {
	var value *apiv1.HttpHeaderSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.HttpHeader.GetKey()
		value_ := event.HttpHeader.GetValue()
		enabled := event.HttpHeader.GetEnabled()
		description := event.HttpHeader.GetDescription()
		order := event.HttpHeader.GetOrder()
		value = &apiv1.HttpHeaderSync_ValueUnion{
			Kind: apiv1.HttpHeaderSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpHeaderSyncInsert{
				HttpHeaderId: event.HttpHeader.GetHttpHeaderId(),
				HttpId:       event.HttpHeader.GetHttpId(),
				Key:          key,
				Value:        value_,
				Enabled:      enabled,
				Description:  description,
				Order:        order,
			},
		}
	case eventTypeUpdate:
		key := event.HttpHeader.GetKey()
		value_ := event.HttpHeader.GetValue()
		enabled := event.HttpHeader.GetEnabled()
		description := event.HttpHeader.GetDescription()
		order := event.HttpHeader.GetOrder()
		value = &apiv1.HttpHeaderSync_ValueUnion{
			Kind: apiv1.HttpHeaderSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpHeaderSyncUpdate{
				HttpHeaderId: event.HttpHeader.GetHttpHeaderId(),
				Key:          &key,
				Value:        &value_,
				Enabled:      &enabled,
				Description:  &description,
				Order:        &order,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpHeaderSync_ValueUnion{
			Kind: apiv1.HttpHeaderSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpHeaderSyncDelete{
				HttpHeaderId: event.HttpHeader.GetHttpHeaderId(),
			},
		}
	}

	return &apiv1.HttpHeaderSyncResponse{
		Items: []*apiv1.HttpHeaderSync{
			{
				Value: value,
			},
		},
	}
}

// httpSearchParamSyncResponseFrom converts HttpSearchParamEvent to HttpSearchParamSync response
func httpSearchParamSyncResponseFrom(event HttpSearchParamEvent) *apiv1.HttpSearchParamSyncResponse {
	var value *apiv1.HttpSearchParamSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.HttpSearchParam.GetKey()
		value_ := event.HttpSearchParam.GetValue()
		enabled := event.HttpSearchParam.GetEnabled()
		description := event.HttpSearchParam.GetDescription()
		order := event.HttpSearchParam.GetOrder()
		value = &apiv1.HttpSearchParamSync_ValueUnion{
			Kind: apiv1.HttpSearchParamSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpSearchParamSyncInsert{
				HttpSearchParamId: event.HttpSearchParam.GetHttpSearchParamId(),
				HttpId:            event.HttpSearchParam.GetHttpId(),
				Key:               key,
				Value:             value_,
				Enabled:           enabled,
				Description:       description,
				Order:             order,
			},
		}
	case eventTypeUpdate:
		key := event.HttpSearchParam.GetKey()
		value_ := event.HttpSearchParam.GetValue()
		enabled := event.HttpSearchParam.GetEnabled()
		description := event.HttpSearchParam.GetDescription()
		order := event.HttpSearchParam.GetOrder()
		value = &apiv1.HttpSearchParamSync_ValueUnion{
			Kind: apiv1.HttpSearchParamSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpSearchParamSyncUpdate{
				HttpSearchParamId: event.HttpSearchParam.GetHttpSearchParamId(),
				Key:               &key,
				Value:             &value_,
				Enabled:           &enabled,
				Description:       &description,
				Order:             &order,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpSearchParamSync_ValueUnion{
			Kind: apiv1.HttpSearchParamSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpSearchParamSyncDelete{
				HttpSearchParamId: event.HttpSearchParam.GetHttpSearchParamId(),
			},
		}
	}

	return &apiv1.HttpSearchParamSyncResponse{
		Items: []*apiv1.HttpSearchParamSync{
			{
				Value: value,
			},
		},
	}
}

// httpAssertSyncResponseFrom converts HttpAssertEvent to HttpAssertSync response
func httpAssertSyncResponseFrom(event HttpAssertEvent) *apiv1.HttpAssertSyncResponse {
	var value *apiv1.HttpAssertSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value_ := event.HttpAssert.GetValue()
		value = &apiv1.HttpAssertSync_ValueUnion{
			Kind: apiv1.HttpAssertSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpAssertSyncInsert{
				HttpAssertId: event.HttpAssert.GetHttpAssertId(),
				HttpId:       event.HttpAssert.GetHttpId(),
				Value:        value_,
			},
		}
	case eventTypeUpdate:
		value_ := event.HttpAssert.GetValue()
		value = &apiv1.HttpAssertSync_ValueUnion{
			Kind: apiv1.HttpAssertSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpAssertSyncUpdate{
				HttpAssertId: event.HttpAssert.GetHttpAssertId(),
				Value:        &value_,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpAssertSync_ValueUnion{
			Kind: apiv1.HttpAssertSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpAssertSyncDelete{
				HttpAssertId: event.HttpAssert.GetHttpAssertId(),
			},
		}
	}

	return &apiv1.HttpAssertSyncResponse{
		Items: []*apiv1.HttpAssertSync{
			{
				Value: value,
			},
		},
	}
}

// httpVersionSyncResponseFrom converts HttpVersionEvent to HttpVersionSync response
func httpVersionSyncResponseFrom(event HttpVersionEvent) *apiv1.HttpVersionSyncResponse {
	var value *apiv1.HttpVersionSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value = &apiv1.HttpVersionSync_ValueUnion{
			Kind: apiv1.HttpVersionSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpVersionSyncInsert{
				HttpVersionId: event.HttpVersion.GetHttpVersionId(),
				HttpId:        event.HttpVersion.GetHttpId(),
				Name:          event.HttpVersion.GetName(),
				Description:   event.HttpVersion.GetDescription(),
				CreatedAt:     event.HttpVersion.GetCreatedAt(),
			},
		}
	case eventTypeUpdate:
		value = &apiv1.HttpVersionSync_ValueUnion{
			Kind: apiv1.HttpVersionSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpVersionSyncUpdate{
				HttpVersionId: event.HttpVersion.GetHttpVersionId(),
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpVersionSync_ValueUnion{
			Kind: apiv1.HttpVersionSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpVersionSyncDelete{
				HttpVersionId: event.HttpVersion.GetHttpVersionId(),
			},
		}
	}

	return &apiv1.HttpVersionSyncResponse{
		Items: []*apiv1.HttpVersionSync{
			{
				Value: value,
			},
		},
	}
}

// httpResponseSyncResponseFrom converts HttpResponseEvent to HttpResponseSync response
func httpResponseSyncResponseFrom(event HttpResponseEvent) *apiv1.HttpResponseSyncResponse {
	var value *apiv1.HttpResponseSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		status := event.HttpResponse.GetStatus()
		body := event.HttpResponse.GetBody()
		time := event.HttpResponse.GetTime()
		duration := event.HttpResponse.GetDuration()
		size := event.HttpResponse.GetSize()
		value = &apiv1.HttpResponseSync_ValueUnion{
			Kind: apiv1.HttpResponseSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpResponseSyncInsert{
				HttpResponseId: event.HttpResponse.GetHttpResponseId(),
				HttpId:         event.HttpResponse.GetHttpId(),
				Status:         status,
				Body:           body,
				Time:           time,
				Duration:       duration,
				Size:           size,
			},
		}
	case eventTypeUpdate:
		status := event.HttpResponse.GetStatus()
		body := event.HttpResponse.GetBody()
		time := event.HttpResponse.GetTime()
		duration := event.HttpResponse.GetDuration()
		size := event.HttpResponse.GetSize()
		value = &apiv1.HttpResponseSync_ValueUnion{
			Kind: apiv1.HttpResponseSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpResponseSyncUpdate{
				HttpResponseId: event.HttpResponse.GetHttpResponseId(),
				Status:         &status,
				Body:           &body,
				Time:           time,
				Duration:       &duration,
				Size:           &size,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpResponseSync_ValueUnion{
			Kind: apiv1.HttpResponseSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpResponseSyncDelete{
				HttpResponseId: event.HttpResponse.GetHttpResponseId(),
			},
		}
	}

	return &apiv1.HttpResponseSyncResponse{
		Items: []*apiv1.HttpResponseSync{
			{
				Value: value,
			},
		},
	}
}

// httpResponseHeaderSyncResponseFrom converts HttpResponseHeaderEvent to HttpResponseHeaderSync response
func httpResponseHeaderSyncResponseFrom(event HttpResponseHeaderEvent) *apiv1.HttpResponseHeaderSyncResponse {
	var value *apiv1.HttpResponseHeaderSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.HttpResponseHeader.GetKey()
		value_ := event.HttpResponseHeader.GetValue()
		value = &apiv1.HttpResponseHeaderSync_ValueUnion{
			Kind: apiv1.HttpResponseHeaderSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpResponseHeaderSyncInsert{
				HttpResponseHeaderId: event.HttpResponseHeader.GetHttpResponseHeaderId(),
				HttpResponseId:       event.HttpResponseHeader.GetHttpResponseId(),
				Key:                  key,
				Value:                value_,
			},
		}
	case eventTypeUpdate:
		key := event.HttpResponseHeader.GetKey()
		value_ := event.HttpResponseHeader.GetValue()
		value = &apiv1.HttpResponseHeaderSync_ValueUnion{
			Kind: apiv1.HttpResponseHeaderSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpResponseHeaderSyncUpdate{
				HttpResponseHeaderId: event.HttpResponseHeader.GetHttpResponseHeaderId(),
				Key:                  &key,
				Value:                &value_,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpResponseHeaderSync_ValueUnion{
			Kind: apiv1.HttpResponseHeaderSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpResponseHeaderSyncDelete{
				HttpResponseHeaderId: event.HttpResponseHeader.GetHttpResponseHeaderId(),
			},
		}
	}

	return &apiv1.HttpResponseHeaderSyncResponse{
		Items: []*apiv1.HttpResponseHeaderSync{
			{
				Value: value,
			},
		},
	}
}

// httpResponseAssertSyncResponseFrom converts HttpResponseAssertEvent to HttpResponseAssertSync response
func httpResponseAssertSyncResponseFrom(event HttpResponseAssertEvent) *apiv1.HttpResponseAssertSyncResponse {
	var value *apiv1.HttpResponseAssertSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value_ := event.HttpResponseAssert.GetValue()
		success := event.HttpResponseAssert.GetSuccess()
		value = &apiv1.HttpResponseAssertSync_ValueUnion{
			Kind: apiv1.HttpResponseAssertSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpResponseAssertSyncInsert{
				HttpResponseAssertId: event.HttpResponseAssert.GetHttpResponseAssertId(),
				HttpResponseId:       event.HttpResponseAssert.GetHttpResponseId(),
				Value:                value_,
				Success:              success,
			},
		}
	case eventTypeUpdate:
		value_ := event.HttpResponseAssert.GetValue()
		success := event.HttpResponseAssert.GetSuccess()
		value = &apiv1.HttpResponseAssertSync_ValueUnion{
			Kind: apiv1.HttpResponseAssertSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpResponseAssertSyncUpdate{
				HttpResponseAssertId: event.HttpResponseAssert.GetHttpResponseAssertId(),
				Value:                &value_,
				Success:              &success,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpResponseAssertSync_ValueUnion{
			Kind: apiv1.HttpResponseAssertSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpResponseAssertSyncDelete{
				HttpResponseAssertId: event.HttpResponseAssert.GetHttpResponseAssertId(),
			},
		}
	}

	return &apiv1.HttpResponseAssertSyncResponse{
		Items: []*apiv1.HttpResponseAssertSync{
			{
				Value: value,
			},
		},
	}
}

// httpBodyRawSyncResponseFrom converts HttpBodyRawEvent to HttpBodyRawSync response
func httpBodyRawSyncResponseFrom(event HttpBodyRawEvent) *apiv1.HttpBodyRawSyncResponse {
	var value *apiv1.HttpBodyRawSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		data := event.HttpBodyRaw.GetData()
		value = &apiv1.HttpBodyRawSync_ValueUnion{
			Kind: apiv1.HttpBodyRawSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpBodyRawSyncInsert{
				HttpId: event.HttpBodyRaw.GetHttpId(),
				Data:   data,
			},
		}
	case eventTypeUpdate:
		data := event.HttpBodyRaw.GetData()
		value = &apiv1.HttpBodyRawSync_ValueUnion{
			Kind: apiv1.HttpBodyRawSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpBodyRawSyncUpdate{
				HttpId: event.HttpBodyRaw.GetHttpId(),
				Data:   &data,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyRawSync_ValueUnion{
			Kind: apiv1.HttpBodyRawSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyRawSyncDelete{
				HttpId: event.HttpBodyRaw.GetHttpId(),
			},
		}
	}

	return &apiv1.HttpBodyRawSyncResponse{
		Items: []*apiv1.HttpBodyRawSync{
			{
				Value: value,
			},
		},
	}
}

// httpDeltaSyncResponseFrom converts HttpEvent to HttpDeltaSync response
func httpDeltaSyncResponseFrom(event HttpEvent, http mhttp.HTTP) *apiv1.HttpDeltaSyncResponse {
	var value *apiv1.HttpDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpDeltaSyncInsert{
			DeltaHttpId: http.ID.Bytes(),
		}
		if http.ParentHttpID != nil {
			delta.HttpId = http.ParentHttpID.Bytes()
		}
		if http.DeltaName != nil {
			delta.Name = http.DeltaName
		}
		if http.DeltaMethod != nil {
			method := converter.ToAPIHttpMethod(*http.DeltaMethod)
			delta.Method = &method
		}
		if http.DeltaUrl != nil {
			delta.Url = http.DeltaUrl
		}
		// Note: BodyKind delta not implemented yet
		value = &apiv1.HttpDeltaSync_ValueUnion{
			Kind:   apiv1.HttpDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpDeltaSyncUpdate{
			DeltaHttpId: http.ID.Bytes(),
		}
		if http.ParentHttpID != nil {
			delta.HttpId = http.ParentHttpID.Bytes()
		}
		if http.DeltaName != nil {
			nameStr := *http.DeltaName
			delta.Name = &apiv1.HttpDeltaSyncUpdate_NameUnion{
				Kind:  apiv1.HttpDeltaSyncUpdate_NameUnion_KIND_VALUE,
				Value: &nameStr,
			}
		} else {
			delta.Name = &apiv1.HttpDeltaSyncUpdate_NameUnion{
				Kind: apiv1.HttpDeltaSyncUpdate_NameUnion_KIND_UNSET,
			}
		}
		if http.DeltaMethod != nil {
			method := converter.ToAPIHttpMethod(*http.DeltaMethod)
			delta.Method = &apiv1.HttpDeltaSyncUpdate_MethodUnion{
				Kind:  apiv1.HttpDeltaSyncUpdate_MethodUnion_KIND_VALUE,
				Value: &method,
			}
		} else {
			delta.Method = &apiv1.HttpDeltaSyncUpdate_MethodUnion{
				Kind: apiv1.HttpDeltaSyncUpdate_MethodUnion_KIND_UNSET,
			}
		}
		if http.DeltaUrl != nil {
			urlStr := *http.DeltaUrl
			delta.Url = &apiv1.HttpDeltaSyncUpdate_UrlUnion{
				Kind:  apiv1.HttpDeltaSyncUpdate_UrlUnion_KIND_VALUE,
				Value: &urlStr,
			}
		} else {
			delta.Url = &apiv1.HttpDeltaSyncUpdate_UrlUnion{
				Kind: apiv1.HttpDeltaSyncUpdate_UrlUnion_KIND_UNSET,
			}
		}
		// Note: BodyKind delta not implemented yet
		value = &apiv1.HttpDeltaSync_ValueUnion{
			Kind:   apiv1.HttpDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpDeltaSync_ValueUnion{
			Kind: apiv1.HttpDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpDeltaSyncDelete{
				DeltaHttpId: http.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpDeltaSyncResponse{
		Items: []*apiv1.HttpDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// HttpSync handles real-time synchronization for HTTP entries
func httpBodyFormDataSyncResponseFrom(event HttpBodyFormEvent) *apiv1.HttpBodyFormDataSyncResponse {
	var value *apiv1.HttpBodyFormDataSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.HttpBodyForm.GetKey()
		value_ := event.HttpBodyForm.GetValue()
		enabled := event.HttpBodyForm.GetEnabled()
		description := event.HttpBodyForm.GetDescription()
		order := event.HttpBodyForm.GetOrder()
		value = &apiv1.HttpBodyFormDataSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpBodyFormDataSyncInsert{
				HttpBodyFormDataId: event.HttpBodyForm.GetHttpBodyFormDataId(),
				HttpId:             event.HttpBodyForm.GetHttpId(),
				Key:                key,
				Value:              value_,
				Enabled:            enabled,
				Description:        description,
				Order:              order,
			},
		}
	case eventTypeUpdate:
		value = &apiv1.HttpBodyFormDataSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpBodyFormDataSyncUpdate{
				HttpBodyFormDataId: event.HttpBodyForm.GetHttpBodyFormDataId(),
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyFormDataSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyFormDataSyncDelete{
				HttpBodyFormDataId: event.HttpBodyForm.GetHttpBodyFormDataId(),
			},
		}
	}

	return &apiv1.HttpBodyFormDataSyncResponse{
		Items: []*apiv1.HttpBodyFormDataSync{
			{
				Value: value,
			},
		},
	}
}

// streamHttpSearchParamDeltaSync streams HTTP search param delta events to the client
func httpBodyUrlEncodedSyncResponseFrom(event HttpBodyUrlEncodedEvent) *apiv1.HttpBodyUrlEncodedSyncResponse {
	var value *apiv1.HttpBodyUrlEncodedSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.HttpBodyUrlEncoded.GetKey()
		value_ := event.HttpBodyUrlEncoded.GetValue()
		enabled := event.HttpBodyUrlEncoded.GetEnabled()
		description := event.HttpBodyUrlEncoded.GetDescription()
		order := event.HttpBodyUrlEncoded.GetOrder()
		value = &apiv1.HttpBodyUrlEncodedSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpBodyUrlEncodedSyncInsert{
				HttpBodyUrlEncodedId: event.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId(),
				HttpId:               event.HttpBodyUrlEncoded.GetHttpId(),
				Key:                  key,
				Value:                value_,
				Enabled:              enabled,
				Description:          description,
				Order:                order,
			},
		}
	case eventTypeUpdate:
		value = &apiv1.HttpBodyUrlEncodedSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpBodyUrlEncodedSyncUpdate{
				HttpBodyUrlEncodedId: event.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId(),
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyUrlEncodedSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyUrlEncodedSyncDelete{
				HttpBodyUrlEncodedId: event.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId(),
			},
		}
	}

	return &apiv1.HttpBodyUrlEncodedSyncResponse{
		Items: []*apiv1.HttpBodyUrlEncodedSync{
			{
				Value: value,
			},
		},
	}
}

// httpSearchParamDeltaSyncResponseFrom converts HttpSearchParamEvent and param record to HttpSearchParamDeltaSync response
func httpSearchParamDeltaSyncResponseFrom(event HttpSearchParamEvent, param mhttp.HTTPSearchParam) *apiv1.HttpSearchParamDeltaSyncResponse {
	var value *apiv1.HttpSearchParamDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpSearchParamDeltaSyncInsert{
			DeltaHttpSearchParamId: param.ID.Bytes(),
		}
		if param.ParentHttpSearchParamID != nil {
			delta.HttpSearchParamId = param.ParentHttpSearchParamID.Bytes()
		}
		delta.HttpId = param.HttpID.Bytes()
		if param.DeltaKey != nil {
			delta.Key = param.DeltaKey
		}
		if param.DeltaValue != nil {
			delta.Value = param.DeltaValue
		}
		if param.DeltaEnabled != nil {
			delta.Enabled = param.DeltaEnabled
		}
		if param.DeltaDescription != nil {
			delta.Description = param.DeltaDescription
		}
		if param.DeltaOrder != nil {
			order := float32(*param.DeltaOrder)
			delta.Order = &order
		}
		value = &apiv1.HttpSearchParamDeltaSync_ValueUnion{
			Kind:   apiv1.HttpSearchParamDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpSearchParamDeltaSyncUpdate{
			DeltaHttpSearchParamId: param.ID.Bytes(),
		}
		if param.ParentHttpSearchParamID != nil {
			delta.HttpSearchParamId = param.ParentHttpSearchParamID.Bytes()
		}
		delta.HttpId = param.HttpID.Bytes()
		if param.DeltaKey != nil {
			keyStr := *param.DeltaKey
			delta.Key = &apiv1.HttpSearchParamDeltaSyncUpdate_KeyUnion{
				Kind:  apiv1.HttpSearchParamDeltaSyncUpdate_KeyUnion_KIND_VALUE,
				Value: &keyStr,
			}
		} else {
			delta.Key = &apiv1.HttpSearchParamDeltaSyncUpdate_KeyUnion{
				Kind: apiv1.HttpSearchParamDeltaSyncUpdate_KeyUnion_KIND_UNSET,
			}
		}
		if param.DeltaValue != nil {
			valueStr := *param.DeltaValue
			delta.Value = &apiv1.HttpSearchParamDeltaSyncUpdate_ValueUnion{
				Kind:  apiv1.HttpSearchParamDeltaSyncUpdate_ValueUnion_KIND_VALUE,
				Value: &valueStr,
			}
		} else {
			delta.Value = &apiv1.HttpSearchParamDeltaSyncUpdate_ValueUnion{
				Kind: apiv1.HttpSearchParamDeltaSyncUpdate_ValueUnion_KIND_UNSET,
			}
		}
		if param.DeltaEnabled != nil {
			enabledBool := *param.DeltaEnabled
			delta.Enabled = &apiv1.HttpSearchParamDeltaSyncUpdate_EnabledUnion{
				Kind:  apiv1.HttpSearchParamDeltaSyncUpdate_EnabledUnion_KIND_VALUE,
				Value: &enabledBool,
			}
		} else {
			delta.Enabled = &apiv1.HttpSearchParamDeltaSyncUpdate_EnabledUnion{
				Kind: apiv1.HttpSearchParamDeltaSyncUpdate_EnabledUnion_KIND_UNSET,
			}
		}
		if param.DeltaDescription != nil {
			descStr := *param.DeltaDescription
			delta.Description = &apiv1.HttpSearchParamDeltaSyncUpdate_DescriptionUnion{
				Kind:  apiv1.HttpSearchParamDeltaSyncUpdate_DescriptionUnion_KIND_VALUE,
				Value: &descStr,
			}
		} else {
			delta.Description = &apiv1.HttpSearchParamDeltaSyncUpdate_DescriptionUnion{
				Kind: apiv1.HttpSearchParamDeltaSyncUpdate_DescriptionUnion_KIND_UNSET,
			}
		}
		if param.DeltaOrder != nil {
			orderFloat := float32(*param.DeltaOrder)
			delta.Order = &apiv1.HttpSearchParamDeltaSyncUpdate_OrderUnion{
				Kind:  apiv1.HttpSearchParamDeltaSyncUpdate_OrderUnion_KIND_VALUE,
				Value: &orderFloat,
			}
		} else {
			delta.Order = &apiv1.HttpSearchParamDeltaSyncUpdate_OrderUnion{
				Kind: apiv1.HttpSearchParamDeltaSyncUpdate_OrderUnion_KIND_UNSET,
			}
		}
		value = &apiv1.HttpSearchParamDeltaSync_ValueUnion{
			Kind:   apiv1.HttpSearchParamDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpSearchParamDeltaSync_ValueUnion{
			Kind: apiv1.HttpSearchParamDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpSearchParamDeltaSyncDelete{
				DeltaHttpSearchParamId: param.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpSearchParamDeltaSyncResponse{
		Items: []*apiv1.HttpSearchParamDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpHeaderDeltaSyncResponseFrom converts HttpHeaderEvent and header record to HttpHeaderDeltaSync response
func httpHeaderDeltaSyncResponseFrom(event HttpHeaderEvent, header mhttp.HTTPHeader) *apiv1.HttpHeaderDeltaSyncResponse {
	var value *apiv1.HttpHeaderDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpHeaderDeltaSyncInsert{
			DeltaHttpHeaderId: header.ID.Bytes(),
		}
		if header.ParentHttpHeaderID != nil {
			delta.HttpHeaderId = header.ParentHttpHeaderID.Bytes()
		}
		delta.HttpId = header.HttpID.Bytes()
		if header.DeltaKey != nil {
			delta.Key = header.DeltaKey
		}
		if header.DeltaValue != nil {
			delta.Value = header.DeltaValue
		}
		if header.DeltaEnabled != nil {
			delta.Enabled = header.DeltaEnabled
		}
		if header.DeltaDescription != nil {
			delta.Description = header.DeltaDescription
		}
		if header.DeltaOrder != nil {
			delta.Order = header.DeltaOrder
		}
		value = &apiv1.HttpHeaderDeltaSync_ValueUnion{
			Kind:   apiv1.HttpHeaderDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpHeaderDeltaSyncUpdate{
			DeltaHttpHeaderId: header.ID.Bytes(),
		}
		if header.ParentHttpHeaderID != nil {
			delta.HttpHeaderId = header.ParentHttpHeaderID.Bytes()
		}
		delta.HttpId = header.HttpID.Bytes()
		if header.DeltaKey != nil {
			keyStr := *header.DeltaKey
			delta.Key = &apiv1.HttpHeaderDeltaSyncUpdate_KeyUnion{
				Kind:  apiv1.HttpHeaderDeltaSyncUpdate_KeyUnion_KIND_VALUE,
				Value: &keyStr,
			}
		} else {
			delta.Key = &apiv1.HttpHeaderDeltaSyncUpdate_KeyUnion{
				Kind: apiv1.HttpHeaderDeltaSyncUpdate_KeyUnion_KIND_UNSET,
			}
		}
		if header.DeltaValue != nil {
			valueStr := *header.DeltaValue
			delta.Value = &apiv1.HttpHeaderDeltaSyncUpdate_ValueUnion{
				Kind:  apiv1.HttpHeaderDeltaSyncUpdate_ValueUnion_KIND_VALUE,
				Value: &valueStr,
			}
		} else {
			delta.Value = &apiv1.HttpHeaderDeltaSyncUpdate_ValueUnion{
				Kind: apiv1.HttpHeaderDeltaSyncUpdate_ValueUnion_KIND_UNSET,
			}
		}
		if header.DeltaEnabled != nil {
			enabledBool := *header.DeltaEnabled
			delta.Enabled = &apiv1.HttpHeaderDeltaSyncUpdate_EnabledUnion{
				Kind:  apiv1.HttpHeaderDeltaSyncUpdate_EnabledUnion_KIND_VALUE,
				Value: &enabledBool,
			}
		} else {
			delta.Enabled = &apiv1.HttpHeaderDeltaSyncUpdate_EnabledUnion{
				Kind: apiv1.HttpHeaderDeltaSyncUpdate_EnabledUnion_KIND_UNSET,
			}
		}
		if header.DeltaDescription != nil {
			descStr := *header.DeltaDescription
			delta.Description = &apiv1.HttpHeaderDeltaSyncUpdate_DescriptionUnion{
				Kind:  apiv1.HttpHeaderDeltaSyncUpdate_DescriptionUnion_KIND_VALUE,
				Value: &descStr,
			}
		} else {
			delta.Description = &apiv1.HttpHeaderDeltaSyncUpdate_DescriptionUnion{
				Kind: apiv1.HttpHeaderDeltaSyncUpdate_DescriptionUnion_KIND_UNSET,
			}
		}
		if header.DeltaOrder != nil {
			orderFloat := *header.DeltaOrder
			delta.Order = &apiv1.HttpHeaderDeltaSyncUpdate_OrderUnion{
				Kind:  apiv1.HttpHeaderDeltaSyncUpdate_OrderUnion_KIND_VALUE,
				Value: &orderFloat,
			}
		} else {
			delta.Order = &apiv1.HttpHeaderDeltaSyncUpdate_OrderUnion{
				Kind: apiv1.HttpHeaderDeltaSyncUpdate_OrderUnion_KIND_UNSET,
			}
		}
		value = &apiv1.HttpHeaderDeltaSync_ValueUnion{
			Kind:   apiv1.HttpHeaderDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpHeaderDeltaSync_ValueUnion{
			Kind: apiv1.HttpHeaderDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpHeaderDeltaSyncDelete{
				DeltaHttpHeaderId: header.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpHeaderDeltaSyncResponse{
		Items: []*apiv1.HttpHeaderDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpBodyFormDeltaSyncResponseFrom converts HttpBodyFormEvent and form record to HttpBodyFormDeltaSync response
func httpBodyFormDataDeltaSyncResponseFrom(event HttpBodyFormEvent, form mhttp.HTTPBodyForm) *apiv1.HttpBodyFormDataDeltaSyncResponse {
	var value *apiv1.HttpBodyFormDataDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpBodyFormDataDeltaSyncInsert{
			DeltaHttpBodyFormDataId: form.ID.Bytes(),
		}
		if form.ParentHttpBodyFormID != nil {
			delta.HttpBodyFormDataId = form.ParentHttpBodyFormID.Bytes()
		}
		delta.HttpId = form.HttpID.Bytes()
		if form.DeltaKey != nil {
			delta.Key = form.DeltaKey
		}
		if form.DeltaValue != nil {
			delta.Value = form.DeltaValue
		}
		if form.DeltaEnabled != nil {
			delta.Enabled = form.DeltaEnabled
		}
		if form.DeltaDescription != nil {
			delta.Description = form.DeltaDescription
		}
		if form.DeltaOrder != nil {
			delta.Order = form.DeltaOrder
		}
		value = &apiv1.HttpBodyFormDataDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyFormDataDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpBodyFormDataDeltaSyncUpdate{
			DeltaHttpBodyFormDataId: form.ID.Bytes(),
		}
		if form.ParentHttpBodyFormID != nil {
			delta.HttpBodyFormDataId = form.ParentHttpBodyFormID.Bytes()
		}
		delta.HttpId = form.HttpID.Bytes()
		if form.DeltaKey != nil {
			keyStr := *form.DeltaKey
			delta.Key = &apiv1.HttpBodyFormDataDeltaSyncUpdate_KeyUnion{
				Kind:  apiv1.HttpBodyFormDataDeltaSyncUpdate_KeyUnion_KIND_VALUE,
				Value: &keyStr,
			}
		} else {
			delta.Key = &apiv1.HttpBodyFormDataDeltaSyncUpdate_KeyUnion{
				Kind: apiv1.HttpBodyFormDataDeltaSyncUpdate_KeyUnion_KIND_UNSET,
			}
		}
		if form.DeltaValue != nil {
			valueStr := *form.DeltaValue
			delta.Value = &apiv1.HttpBodyFormDataDeltaSyncUpdate_ValueUnion{
				Kind:  apiv1.HttpBodyFormDataDeltaSyncUpdate_ValueUnion_KIND_VALUE,
				Value: &valueStr,
			}
		} else {
			delta.Value = &apiv1.HttpBodyFormDataDeltaSyncUpdate_ValueUnion{
				Kind: apiv1.HttpBodyFormDataDeltaSyncUpdate_ValueUnion_KIND_UNSET,
			}
		}
		if form.DeltaEnabled != nil {
			enabledBool := *form.DeltaEnabled
			delta.Enabled = &apiv1.HttpBodyFormDataDeltaSyncUpdate_EnabledUnion{
				Kind:  apiv1.HttpBodyFormDataDeltaSyncUpdate_EnabledUnion_KIND_VALUE,
				Value: &enabledBool,
			}
		} else {
			delta.Enabled = &apiv1.HttpBodyFormDataDeltaSyncUpdate_EnabledUnion{
				Kind: apiv1.HttpBodyFormDataDeltaSyncUpdate_EnabledUnion_KIND_UNSET,
			}
		}
		if form.DeltaDescription != nil {
			descStr := *form.DeltaDescription
			delta.Description = &apiv1.HttpBodyFormDataDeltaSyncUpdate_DescriptionUnion{
				Kind:  apiv1.HttpBodyFormDataDeltaSyncUpdate_DescriptionUnion_KIND_VALUE,
				Value: &descStr,
			}
		} else {
			delta.Description = &apiv1.HttpBodyFormDataDeltaSyncUpdate_DescriptionUnion{
				Kind: apiv1.HttpBodyFormDataDeltaSyncUpdate_DescriptionUnion_KIND_UNSET,
			}
		}
		if form.DeltaOrder != nil {
			orderFloat := *form.DeltaOrder
			delta.Order = &apiv1.HttpBodyFormDataDeltaSyncUpdate_OrderUnion{
				Kind:  apiv1.HttpBodyFormDataDeltaSyncUpdate_OrderUnion_KIND_VALUE,
				Value: &orderFloat,
			}
		} else {
			delta.Order = &apiv1.HttpBodyFormDataDeltaSyncUpdate_OrderUnion{
				Kind: apiv1.HttpBodyFormDataDeltaSyncUpdate_OrderUnion_KIND_UNSET,
			}
		}
		value = &apiv1.HttpBodyFormDataDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyFormDataDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyFormDataDeltaSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyFormDataDeltaSyncDelete{
				DeltaHttpBodyFormDataId: form.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpBodyFormDataDeltaSyncResponse{
		Items: []*apiv1.HttpBodyFormDataDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpAssertDeltaSyncResponseFrom converts HttpAssertEvent and assert record to HttpAssertDeltaSync response
func httpAssertDeltaSyncResponseFrom(event HttpAssertEvent, assert mhttpassert.HttpAssert) *apiv1.HttpAssertDeltaSyncResponse {
	var value *apiv1.HttpAssertDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpAssertDeltaSyncInsert{
			DeltaHttpAssertId: assert.ID.Bytes(),
		}
		if assert.ParentHttpAssertID != nil {
			delta.HttpAssertId = assert.ParentHttpAssertID.Bytes()
		}
		delta.HttpId = assert.HttpID.Bytes()
		if assert.DeltaValue != nil {
			delta.Value = assert.DeltaValue
		}
		value = &apiv1.HttpAssertDeltaSync_ValueUnion{
			Kind:   apiv1.HttpAssertDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpAssertDeltaSyncUpdate{
			DeltaHttpAssertId: assert.ID.Bytes(),
		}
		if assert.ParentHttpAssertID != nil {
			delta.HttpAssertId = assert.ParentHttpAssertID.Bytes()
		}
		delta.HttpId = assert.HttpID.Bytes()
		if assert.DeltaValue != nil {
			valueStr := *assert.DeltaValue
			delta.Value = &apiv1.HttpAssertDeltaSyncUpdate_ValueUnion{
				Kind:  apiv1.HttpAssertDeltaSyncUpdate_ValueUnion_KIND_VALUE,
				Value: &valueStr,
			}
		} else {
			delta.Value = &apiv1.HttpAssertDeltaSyncUpdate_ValueUnion{
				Kind: apiv1.HttpAssertDeltaSyncUpdate_ValueUnion_KIND_UNSET,
			}
		}
		value = &apiv1.HttpAssertDeltaSync_ValueUnion{
			Kind:   apiv1.HttpAssertDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpAssertDeltaSync_ValueUnion{
			Kind: apiv1.HttpAssertDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpAssertDeltaSyncDelete{
				DeltaHttpAssertId: assert.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpAssertDeltaSyncResponse{
		Items: []*apiv1.HttpAssertDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpBodyUrlEncodedDeltaSyncResponseFrom converts HttpBodyUrlEncodedEvent and body record to HttpBodyUrlEncodedDeltaSync response
func httpBodyUrlEncodedDeltaSyncResponseFrom(event HttpBodyUrlEncodedEvent, body mhttp.HTTPBodyUrlencoded) *apiv1.HttpBodyUrlEncodedDeltaSyncResponse {
	var value *apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpBodyUrlEncodedDeltaSyncInsert{
			DeltaHttpBodyUrlEncodedId: body.ID.Bytes(),
		}
		if body.ParentHttpBodyUrlEncodedID != nil {
			delta.HttpBodyUrlEncodedId = body.ParentHttpBodyUrlEncodedID.Bytes()
		}
		delta.HttpId = body.HttpID.Bytes()
		if body.DeltaKey != nil {
			delta.Key = body.DeltaKey
		}
		if body.DeltaValue != nil {
			delta.Value = body.DeltaValue
		}
		if body.DeltaEnabled != nil {
			delta.Enabled = body.DeltaEnabled
		}
		if body.DeltaDescription != nil {
			delta.Description = body.DeltaDescription
		}
		if body.DeltaOrder != nil {
			delta.Order = body.DeltaOrder
		}
		value = &apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate{
			DeltaHttpBodyUrlEncodedId: body.ID.Bytes(),
		}
		if body.ParentHttpBodyUrlEncodedID != nil {
			delta.HttpBodyUrlEncodedId = body.ParentHttpBodyUrlEncodedID.Bytes()
		}
		delta.HttpId = body.HttpID.Bytes()
		if body.DeltaKey != nil {
			keyStr := *body.DeltaKey
			delta.Key = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_KeyUnion{
				Kind:  apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_KeyUnion_KIND_VALUE,
				Value: &keyStr,
			}
		} else {
			delta.Key = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_KeyUnion{
				Kind: apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_KeyUnion_KIND_UNSET,
			}
		}
		if body.DeltaValue != nil {
			valueStr := *body.DeltaValue
			delta.Value = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_ValueUnion{
				Kind:  apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_ValueUnion_KIND_VALUE,
				Value: &valueStr,
			}
		} else {
			delta.Value = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_ValueUnion{
				Kind: apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_ValueUnion_KIND_UNSET,
			}
		}
		if body.DeltaEnabled != nil {
			enabledBool := *body.DeltaEnabled
			delta.Enabled = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_EnabledUnion{
				Kind:  apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_EnabledUnion_KIND_VALUE,
				Value: &enabledBool,
			}
		} else {
			delta.Enabled = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_EnabledUnion{
				Kind: apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_EnabledUnion_KIND_UNSET,
			}
		}
		if body.DeltaDescription != nil {
			descStr := *body.DeltaDescription
			delta.Description = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_DescriptionUnion{
				Kind:  apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_DescriptionUnion_KIND_VALUE,
				Value: &descStr,
			}
		} else {
			delta.Description = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_DescriptionUnion{
				Kind: apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_DescriptionUnion_KIND_UNSET,
			}
		}
		if body.DeltaOrder != nil {
			orderFloat := *body.DeltaOrder
			delta.Order = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_OrderUnion{
				Kind:  apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_OrderUnion_KIND_VALUE,
				Value: &orderFloat,
			}
		} else {
			delta.Order = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_OrderUnion{
				Kind: apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_OrderUnion_KIND_UNSET,
			}
		}
		value = &apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyUrlEncodedDeltaSyncDelete{
				DeltaHttpBodyUrlEncodedId: body.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpBodyUrlEncodedDeltaSyncResponse{
		Items: []*apiv1.HttpBodyUrlEncodedDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// streamHttpBodyUrlEncodedDeltaSync streams HTTP body URL encoded delta events to the client
