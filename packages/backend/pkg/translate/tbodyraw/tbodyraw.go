package tbodyraw

/*

import (
	"context"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/translate/tbodyform"
	"the-dev-tools/backend/pkg/translate/tbodyurl"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/backend/pkg/zstdcompress"
	bodyv1 "the-dev-tools/services/gen/body/v1"

	"connectrpc.com/connect"
)

func SerializeModelToRPC(ctx context.Context, ex mitemapiexample.ItemApiExample, brs *sbodyraw.BodyRawService, bfs *sbodyform.BodyFormService, bues *sbodyurl.BodyURLEncodedService) (*bodyv1.Body, error) {
	var body *bodyv1.Body
	switch ex.BodyType {
	case mitemapiexample.BodyTypeNone:
		body = &bodyv1.Body{
			Value: &bodyv1.Body_None{
				None: &bodyv1.BodyNone{},
			},
		}
	case mitemapiexample.BodyTypeRaw:
		bodyData, err := brs.GetBodyRawByExampleID(ctx, ex.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if bodyData.CompressType == mbodyraw.CompressTypeZstd {
			bodyData.Data, err = zstdcompress.Decompress(bodyData.Data)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		body = &bodyv1.Body{
			Value: &bodyv1.Body_Raw{
				Raw: &bodyv1.BodyRaw{
					BodyBytes: bodyData.Data,
				},
			},
		}
	case mitemapiexample.BodyTypeForm:
		forms, err := bfs.GetBodyFormsByExampleID(ctx, ex.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		body = &bodyv1.Body{
			Value: &bodyv1.Body_Forms{
				Forms: &bodyv1.BodyFormArray{
					Items: tgeneric.MassConvert(forms, tbodyform.SerializeFormModelToRPC),
				},
			},
		}
	case mitemapiexample.BodyTypeUrlencoded:
		urls, err := bues.GetBodyURLEncodedByExampleID(ctx, ex.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		body = &bodyv1.Body{
			Value: &bodyv1.Body_UrlEncodeds{
				UrlEncodeds: &bodyv1.BodyUrlEncodedArray{
					Items: tgeneric.MassConvert(urls, tbodyurl.SerializeURLModelToRPC),
				},
			},
		}
	}
	return body, nil
}
*/
