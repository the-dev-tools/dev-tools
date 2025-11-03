package shttp

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHttpHeaderFound = errors.New("no HTTP header found")

type HttpHeaderService struct {
	queries *gen.Queries
}

func ConvertToDBHttpHeader(header mhttp.HTTPHeader) gen.HttpHeader {
	return gen.HttpHeader{
		ID:               header.ID,
		HttpID:           header.HttpID,
		HeaderKey:        header.HeaderKey,
		HeaderValue:      header.HeaderValue,
		Description:      header.Description,
		Enabled:          header.Enabled,
		ParentHeaderID:   header.ParentHeaderID,
		IsDelta:          header.IsDelta,
		DeltaHeaderKey:   header.DeltaHeaderKey,
		DeltaHeaderValue: header.DeltaHeaderValue,
		DeltaDescription: header.DeltaDescription,
		DeltaEnabled:     header.DeltaEnabled,
		Prev:             header.Prev,
		Next:             header.Next,
		CreatedAt:        header.CreatedAt,
		UpdatedAt:        header.UpdatedAt,
	}
}

func ConvertToModelHttpHeader(header gen.HttpHeader) mhttp.HTTPHeader {
	return mhttp.HTTPHeader{
		ID:               header.ID,
		HttpID:           header.HttpID,
		HeaderKey:        header.HeaderKey,
		HeaderValue:      header.HeaderValue,
		Description:      header.Description,
		Enabled:          header.Enabled,
		ParentHeaderID:   header.ParentHeaderID,
		IsDelta:          header.IsDelta,
		DeltaHeaderKey:   header.DeltaHeaderKey,
		DeltaHeaderValue: header.DeltaHeaderValue,
		DeltaDescription: header.DeltaDescription,
		DeltaEnabled:     header.DeltaEnabled,
		Prev:             header.Prev,
		Next:             header.Next,
		CreatedAt:        header.CreatedAt,
		UpdatedAt:        header.UpdatedAt,
	}
}

func NewHttpHeaderService(queries *gen.Queries) HttpHeaderService {
	return HttpHeaderService{queries: queries}
}

func (hhs HttpHeaderService) TX(tx *sql.Tx) HttpHeaderService {
	return HttpHeaderService{queries: hhs.queries.WithTx(tx)}
}

func NewHttpHeaderServiceTX(ctx context.Context, tx *sql.Tx) (*HttpHeaderService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HttpHeaderService{queries: queries}
	return &service, nil
}

func (hhs HttpHeaderService) CreateBulk(ctx context.Context, headers []mhttp.HTTPHeader) error {
	const sizeOfChunks = 10
	now := dbtime.DBNow().Unix()

	// Set timestamps for all headers
	for i := range headers {
		headers[i].CreatedAt = now
		headers[i].UpdatedAt = now
	}

	convertedItems := tgeneric.MassConvert(headers, ConvertToDBHttpHeader)
	for headerChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(headerChunk) < sizeOfChunks {
			for _, header := range headerChunk {
				err := hhs.Create(ctx, header)
				if err != nil {
					return err
				}
			}
			continue
		}

		item1 := headerChunk[0]
		item2 := headerChunk[1]
		item3 := headerChunk[2]
		item4 := headerChunk[3]
		item5 := headerChunk[4]
		item6 := headerChunk[5]
		item7 := headerChunk[6]
		item8 := headerChunk[7]
		item9 := headerChunk[8]
		item10 := headerChunk[9]

		params := gen.CreateHTTPHeadersBulkParams{
			// 1
			ID:               item1.ID,
			HttpID:           item1.HttpID,
			HeaderKey:        item1.HeaderKey,
			HeaderValue:      item1.HeaderValue,
			Description:      item1.Description,
			Enabled:          item1.Enabled,
			ParentHeaderID:   item1.ParentHeaderID,
			IsDelta:          item1.IsDelta,
			DeltaHeaderKey:   item1.DeltaHeaderKey,
			DeltaHeaderValue: item1.DeltaHeaderValue,
			DeltaDescription: item1.DeltaDescription,
			DeltaEnabled:     item1.DeltaEnabled,
			Prev:             item1.Prev,
			Next:             item1.Next,
			CreatedAt:        item1.CreatedAt,
			UpdatedAt:        item1.UpdatedAt,
			// 2
			ID_2:               item2.ID,
			HttpID_2:           item2.HttpID,
			HeaderKey_2:        item2.HeaderKey,
			HeaderValue_2:      item2.HeaderValue,
			Description_2:      item2.Description,
			Enabled_2:          item2.Enabled,
			ParentHeaderID_2:   item2.ParentHeaderID,
			IsDelta_2:          item2.IsDelta,
			DeltaHeaderKey_2:   item2.DeltaHeaderKey,
			DeltaHeaderValue_2: item2.DeltaHeaderValue,
			DeltaDescription_2: item2.DeltaDescription,
			DeltaEnabled_2:     item2.DeltaEnabled,
			Prev_2:             item2.Prev,
			Next_2:             item2.Next,
			CreatedAt_2:        item2.CreatedAt,
			UpdatedAt_2:        item2.UpdatedAt,
			// 3
			ID_3:               item3.ID,
			HttpID_3:           item3.HttpID,
			HeaderKey_3:        item3.HeaderKey,
			HeaderValue_3:      item3.HeaderValue,
			Description_3:      item3.Description,
			Enabled_3:          item3.Enabled,
			ParentHeaderID_3:   item3.ParentHeaderID,
			IsDelta_3:          item3.IsDelta,
			DeltaHeaderKey_3:   item3.DeltaHeaderKey,
			DeltaHeaderValue_3: item3.DeltaHeaderValue,
			DeltaDescription_3: item3.DeltaDescription,
			DeltaEnabled_3:     item3.DeltaEnabled,
			Prev_3:             item3.Prev,
			Next_3:             item3.Next,
			CreatedAt_3:        item3.CreatedAt,
			UpdatedAt_3:        item3.UpdatedAt,
			// 4
			ID_4:               item4.ID,
			HttpID_4:           item4.HttpID,
			HeaderKey_4:        item4.HeaderKey,
			HeaderValue_4:      item4.HeaderValue,
			Description_4:      item4.Description,
			Enabled_4:          item4.Enabled,
			ParentHeaderID_4:   item4.ParentHeaderID,
			IsDelta_4:          item4.IsDelta,
			DeltaHeaderKey_4:   item4.DeltaHeaderKey,
			DeltaHeaderValue_4: item4.DeltaHeaderValue,
			DeltaDescription_4: item4.DeltaDescription,
			DeltaEnabled_4:     item4.DeltaEnabled,
			Prev_4:             item4.Prev,
			Next_4:             item4.Next,
			CreatedAt_4:        item4.CreatedAt,
			UpdatedAt_4:        item4.UpdatedAt,
			// 5
			ID_5:               item5.ID,
			HttpID_5:           item5.HttpID,
			HeaderKey_5:        item5.HeaderKey,
			HeaderValue_5:      item5.HeaderValue,
			Description_5:      item5.Description,
			Enabled_5:          item5.Enabled,
			ParentHeaderID_5:   item5.ParentHeaderID,
			IsDelta_5:          item5.IsDelta,
			DeltaHeaderKey_5:   item5.DeltaHeaderKey,
			DeltaHeaderValue_5: item5.DeltaHeaderValue,
			DeltaDescription_5: item5.DeltaDescription,
			DeltaEnabled_5:     item5.DeltaEnabled,
			Prev_5:             item5.Prev,
			Next_5:             item5.Next,
			CreatedAt_5:        item5.CreatedAt,
			UpdatedAt_5:        item5.UpdatedAt,
			// 6
			ID_6:               item6.ID,
			HttpID_6:           item6.HttpID,
			HeaderKey_6:        item6.HeaderKey,
			HeaderValue_6:      item6.HeaderValue,
			Description_6:      item6.Description,
			Enabled_6:          item6.Enabled,
			ParentHeaderID_6:   item6.ParentHeaderID,
			IsDelta_6:          item6.IsDelta,
			DeltaHeaderKey_6:   item6.DeltaHeaderKey,
			DeltaHeaderValue_6: item6.DeltaHeaderValue,
			DeltaDescription_6: item6.DeltaDescription,
			DeltaEnabled_6:     item6.DeltaEnabled,
			Prev_6:             item6.Prev,
			Next_6:             item6.Next,
			CreatedAt_6:        item6.CreatedAt,
			UpdatedAt_6:        item6.UpdatedAt,
			// 7
			ID_7:               item7.ID,
			HttpID_7:           item7.HttpID,
			HeaderKey_7:        item7.HeaderKey,
			HeaderValue_7:      item7.HeaderValue,
			Description_7:      item7.Description,
			Enabled_7:          item7.Enabled,
			ParentHeaderID_7:   item7.ParentHeaderID,
			IsDelta_7:          item7.IsDelta,
			DeltaHeaderKey_7:   item7.DeltaHeaderKey,
			DeltaHeaderValue_7: item7.DeltaHeaderValue,
			DeltaDescription_7: item7.DeltaDescription,
			DeltaEnabled_7:     item7.DeltaEnabled,
			Prev_7:             item7.Prev,
			Next_7:             item7.Next,
			CreatedAt_7:        item7.CreatedAt,
			UpdatedAt_7:        item7.UpdatedAt,
			// 8
			ID_8:               item8.ID,
			HttpID_8:           item8.HttpID,
			HeaderKey_8:        item8.HeaderKey,
			HeaderValue_8:      item8.HeaderValue,
			Description_8:      item8.Description,
			Enabled_8:          item8.Enabled,
			ParentHeaderID_8:   item8.ParentHeaderID,
			IsDelta_8:          item8.IsDelta,
			DeltaHeaderKey_8:   item8.DeltaHeaderKey,
			DeltaHeaderValue_8: item8.DeltaHeaderValue,
			DeltaDescription_8: item8.DeltaDescription,
			DeltaEnabled_8:     item8.DeltaEnabled,
			Prev_8:             item8.Prev,
			Next_8:             item8.Next,
			CreatedAt_8:        item8.CreatedAt,
			UpdatedAt_8:        item8.UpdatedAt,
			// 9
			ID_9:               item9.ID,
			HttpID_9:           item9.HttpID,
			HeaderKey_9:        item9.HeaderKey,
			HeaderValue_9:      item9.HeaderValue,
			Description_9:      item9.Description,
			Enabled_9:          item9.Enabled,
			ParentHeaderID_9:   item9.ParentHeaderID,
			IsDelta_9:          item9.IsDelta,
			DeltaHeaderKey_9:   item9.DeltaHeaderKey,
			DeltaHeaderValue_9: item9.DeltaHeaderValue,
			DeltaDescription_9: item9.DeltaDescription,
			DeltaEnabled_9:     item9.DeltaEnabled,
			Prev_9:             item9.Prev,
			Next_9:             item9.Next,
			CreatedAt_9:        item9.CreatedAt,
			UpdatedAt_9:        item9.UpdatedAt,
			// 10
			ID_10:               item10.ID,
			HttpID_10:           item10.HttpID,
			HeaderKey_10:        item10.HeaderKey,
			HeaderValue_10:      item10.HeaderValue,
			Description_10:      item10.Description,
			Enabled_10:          item10.Enabled,
			ParentHeaderID_10:   item10.ParentHeaderID,
			IsDelta_10:          item10.IsDelta,
			DeltaHeaderKey_10:   item10.DeltaHeaderKey,
			DeltaHeaderValue_10: item10.DeltaHeaderValue,
			DeltaDescription_10: item10.DeltaDescription,
			DeltaEnabled_10:     item10.DeltaEnabled,
			Prev_10:             item10.Prev,
			Next_10:             item10.Next,
			CreatedAt_10:        item10.CreatedAt,
			UpdatedAt_10:        item10.UpdatedAt,
		}
		if err := hhs.queries.CreateHTTPHeadersBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (hhs HttpHeaderService) Create(ctx context.Context, header gen.HttpHeader) error {
	return hhs.queries.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams{
		ID:               header.ID,
		HttpID:           header.HttpID,
		HeaderKey:        header.HeaderKey,
		HeaderValue:      header.HeaderValue,
		Description:      header.Description,
		Enabled:          header.Enabled,
		ParentHeaderID:   header.ParentHeaderID,
		IsDelta:          header.IsDelta,
		DeltaHeaderKey:   header.DeltaHeaderKey,
		DeltaHeaderValue: header.DeltaHeaderValue,
		DeltaDescription: header.DeltaDescription,
		DeltaEnabled:     header.DeltaEnabled,
		Prev:             header.Prev,
		Next:             header.Next,
		CreatedAt:        header.CreatedAt,
		UpdatedAt:        header.UpdatedAt,
	})
}

func (hhs HttpHeaderService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	headers, err := hhs.queries.GetHTTPHeaders(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPHeader{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvert(headers, ConvertToModelHttpHeader), nil
}
