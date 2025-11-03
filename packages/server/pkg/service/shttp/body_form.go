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

var ErrNoHttpBodyFormFound = errors.New("no HTTP body form found")

type HttpBodyFormService struct {
	queries *gen.Queries
}

func ConvertToDBHttpBodyForm(form mhttp.HTTPBodyForm) gen.HttpBodyForm {
	return gen.HttpBodyForm{
		ID:               form.ID,
		HttpID:           form.HttpID,
		FormKey:          form.FormKey,
		FormValue:        form.FormValue,
		Description:      form.Description,
		Enabled:          form.Enabled,
		ParentBodyFormID: form.ParentBodyFormID,
		IsDelta:          form.IsDelta,
		DeltaFormKey:     form.DeltaFormKey,
		DeltaFormValue:   form.DeltaFormValue,
		DeltaDescription: form.DeltaDescription,
		DeltaEnabled:     form.DeltaEnabled,
		Prev:             form.Prev,
		Next:             form.Next,
		CreatedAt:        form.CreatedAt,
		UpdatedAt:        form.UpdatedAt,
	}
}

func ConvertToModelHttpBodyForm(form gen.HttpBodyForm) mhttp.HTTPBodyForm {
	return mhttp.HTTPBodyForm{
		ID:               form.ID,
		HttpID:           form.HttpID,
		FormKey:          form.FormKey,
		FormValue:        form.FormValue,
		Description:      form.Description,
		Enabled:          form.Enabled,
		ParentBodyFormID: form.ParentBodyFormID,
		IsDelta:          form.IsDelta,
		DeltaFormKey:     form.DeltaFormKey,
		DeltaFormValue:   form.DeltaFormValue,
		DeltaDescription: form.DeltaDescription,
		DeltaEnabled:     form.DeltaEnabled,
		Prev:             form.Prev,
		Next:             form.Next,
		CreatedAt:        form.CreatedAt,
		UpdatedAt:        form.UpdatedAt,
	}
}

func NewHttpBodyFormService(queries *gen.Queries) HttpBodyFormService {
	return HttpBodyFormService{queries: queries}
}

func (hbfs HttpBodyFormService) TX(tx *sql.Tx) HttpBodyFormService {
	return HttpBodyFormService{queries: hbfs.queries.WithTx(tx)}
}

func NewHttpBodyFormServiceTX(ctx context.Context, tx *sql.Tx) (*HttpBodyFormService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HttpBodyFormService{queries: queries}
	return &service, nil
}

func (hbfs HttpBodyFormService) CreateBulk(ctx context.Context, forms []mhttp.HTTPBodyForm) error {
	const sizeOfChunks = 10
	now := dbtime.DBNow().Unix()

	// Set timestamps for all forms
	for i := range forms {
		forms[i].CreatedAt = now
		forms[i].UpdatedAt = now
	}

	convertedItems := tgeneric.MassConvert(forms, ConvertToDBHttpBodyForm)
	for formChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(formChunk) < sizeOfChunks {
			for _, form := range formChunk {
				err := hbfs.Create(ctx, form)
				if err != nil {
					return err
				}
			}
			continue
		}

		item1 := formChunk[0]
		item2 := formChunk[1]
		item3 := formChunk[2]
		item4 := formChunk[3]
		item5 := formChunk[4]
		item6 := formChunk[5]
		item7 := formChunk[6]
		item8 := formChunk[7]
		item9 := formChunk[8]
		item10 := formChunk[9]

		params := gen.CreateHTTPBodyFormsBulkParams{
			// 1
			ID:               item1.ID,
			HttpID:           item1.HttpID,
			FormKey:          item1.FormKey,
			FormValue:        item1.FormValue,
			Description:      item1.Description,
			Enabled:          item1.Enabled,
			ParentBodyFormID: item1.ParentBodyFormID,
			IsDelta:          item1.IsDelta,
			DeltaFormKey:     item1.DeltaFormKey,
			DeltaFormValue:   item1.DeltaFormValue,
			DeltaDescription: item1.DeltaDescription,
			DeltaEnabled:     item1.DeltaEnabled,
			Prev:             item1.Prev,
			Next:             item1.Next,
			CreatedAt:        item1.CreatedAt,
			UpdatedAt:        item1.UpdatedAt,
			// 2
			ID_2:               item2.ID,
			HttpID_2:           item2.HttpID,
			FormKey_2:          item2.FormKey,
			FormValue_2:        item2.FormValue,
			Description_2:      item2.Description,
			Enabled_2:          item2.Enabled,
			ParentBodyFormID_2: item2.ParentBodyFormID,
			IsDelta_2:          item2.IsDelta,
			DeltaFormKey_2:     item2.DeltaFormKey,
			DeltaFormValue_2:   item2.DeltaFormValue,
			DeltaDescription_2: item2.DeltaDescription,
			DeltaEnabled_2:     item2.DeltaEnabled,
			Prev_2:             item2.Prev,
			Next_2:             item2.Next,
			CreatedAt_2:        item2.CreatedAt,
			UpdatedAt_2:        item2.UpdatedAt,
			// 3
			ID_3:               item3.ID,
			HttpID_3:           item3.HttpID,
			FormKey_3:          item3.FormKey,
			FormValue_3:        item3.FormValue,
			Description_3:      item3.Description,
			Enabled_3:          item3.Enabled,
			ParentBodyFormID_3: item3.ParentBodyFormID,
			IsDelta_3:          item3.IsDelta,
			DeltaFormKey_3:     item3.DeltaFormKey,
			DeltaFormValue_3:   item3.DeltaFormValue,
			DeltaDescription_3: item3.DeltaDescription,
			DeltaEnabled_3:     item3.DeltaEnabled,
			Prev_3:             item3.Prev,
			Next_3:             item3.Next,
			CreatedAt_3:        item3.CreatedAt,
			UpdatedAt_3:        item3.UpdatedAt,
			// 4
			ID_4:               item4.ID,
			HttpID_4:           item4.HttpID,
			FormKey_4:          item4.FormKey,
			FormValue_4:        item4.FormValue,
			Description_4:      item4.Description,
			Enabled_4:          item4.Enabled,
			ParentBodyFormID_4: item4.ParentBodyFormID,
			IsDelta_4:          item4.IsDelta,
			DeltaFormKey_4:     item4.DeltaFormKey,
			DeltaFormValue_4:   item4.DeltaFormValue,
			DeltaDescription_4: item4.DeltaDescription,
			DeltaEnabled_4:     item4.DeltaEnabled,
			Prev_4:             item4.Prev,
			Next_4:             item4.Next,
			CreatedAt_4:        item4.CreatedAt,
			UpdatedAt_4:        item4.UpdatedAt,
			// 5
			ID_5:               item5.ID,
			HttpID_5:           item5.HttpID,
			FormKey_5:          item5.FormKey,
			FormValue_5:        item5.FormValue,
			Description_5:      item5.Description,
			Enabled_5:          item5.Enabled,
			ParentBodyFormID_5: item5.ParentBodyFormID,
			IsDelta_5:          item5.IsDelta,
			DeltaFormKey_5:     item5.DeltaFormKey,
			DeltaFormValue_5:   item5.DeltaFormValue,
			DeltaDescription_5: item5.DeltaDescription,
			DeltaEnabled_5:     item5.DeltaEnabled,
			Prev_5:             item5.Prev,
			Next_5:             item5.Next,
			CreatedAt_5:        item5.CreatedAt,
			UpdatedAt_5:        item5.UpdatedAt,
			// 6
			ID_6:               item6.ID,
			HttpID_6:           item6.HttpID,
			FormKey_6:          item6.FormKey,
			FormValue_6:        item6.FormValue,
			Description_6:      item6.Description,
			Enabled_6:          item6.Enabled,
			ParentBodyFormID_6: item6.ParentBodyFormID,
			IsDelta_6:          item6.IsDelta,
			DeltaFormKey_6:     item6.DeltaFormKey,
			DeltaFormValue_6:   item6.DeltaFormValue,
			DeltaDescription_6: item6.DeltaDescription,
			DeltaEnabled_6:     item6.DeltaEnabled,
			Prev_6:             item6.Prev,
			Next_6:             item6.Next,
			CreatedAt_6:        item6.CreatedAt,
			UpdatedAt_6:        item6.UpdatedAt,
			// 7
			ID_7:               item7.ID,
			HttpID_7:           item7.HttpID,
			FormKey_7:          item7.FormKey,
			FormValue_7:        item7.FormValue,
			Description_7:      item7.Description,
			Enabled_7:          item7.Enabled,
			ParentBodyFormID_7: item7.ParentBodyFormID,
			IsDelta_7:          item7.IsDelta,
			DeltaFormKey_7:     item7.DeltaFormKey,
			DeltaFormValue_7:   item7.DeltaFormValue,
			DeltaDescription_7: item7.DeltaDescription,
			DeltaEnabled_7:     item7.DeltaEnabled,
			Prev_7:             item7.Prev,
			Next_7:             item7.Next,
			CreatedAt_7:        item7.CreatedAt,
			UpdatedAt_7:        item7.UpdatedAt,
			// 8
			ID_8:               item8.ID,
			HttpID_8:           item8.HttpID,
			FormKey_8:          item8.FormKey,
			FormValue_8:        item8.FormValue,
			Description_8:      item8.Description,
			Enabled_8:          item8.Enabled,
			ParentBodyFormID_8: item8.ParentBodyFormID,
			IsDelta_8:          item8.IsDelta,
			DeltaFormKey_8:     item8.DeltaFormKey,
			DeltaFormValue_8:   item8.DeltaFormValue,
			DeltaDescription_8: item8.DeltaDescription,
			DeltaEnabled_8:     item8.DeltaEnabled,
			Prev_8:             item8.Prev,
			Next_8:             item8.Next,
			CreatedAt_8:        item8.CreatedAt,
			UpdatedAt_8:        item8.UpdatedAt,
			// 9
			ID_9:               item9.ID,
			HttpID_9:           item9.HttpID,
			FormKey_9:          item9.FormKey,
			FormValue_9:        item9.FormValue,
			Description_9:      item9.Description,
			Enabled_9:          item9.Enabled,
			ParentBodyFormID_9: item9.ParentBodyFormID,
			IsDelta_9:          item9.IsDelta,
			DeltaFormKey_9:     item9.DeltaFormKey,
			DeltaFormValue_9:   item9.DeltaFormValue,
			DeltaDescription_9: item9.DeltaDescription,
			DeltaEnabled_9:     item9.DeltaEnabled,
			Prev_9:             item9.Prev,
			Next_9:             item9.Next,
			CreatedAt_9:        item9.CreatedAt,
			UpdatedAt_9:        item9.UpdatedAt,
			// 10
			ID_10:               item10.ID,
			HttpID_10:           item10.HttpID,
			FormKey_10:          item10.FormKey,
			FormValue_10:        item10.FormValue,
			Description_10:      item10.Description,
			Enabled_10:          item10.Enabled,
			ParentBodyFormID_10: item10.ParentBodyFormID,
			IsDelta_10:          item10.IsDelta,
			DeltaFormKey_10:     item10.DeltaFormKey,
			DeltaFormValue_10:   item10.DeltaFormValue,
			DeltaDescription_10: item10.DeltaDescription,
			DeltaEnabled_10:     item10.DeltaEnabled,
			Prev_10:             item10.Prev,
			Next_10:             item10.Next,
			CreatedAt_10:        item10.CreatedAt,
			UpdatedAt_10:        item10.UpdatedAt,
		}
		if err := hbfs.queries.CreateHTTPBodyFormsBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (hbfs HttpBodyFormService) Create(ctx context.Context, form gen.HttpBodyForm) error {
	return hbfs.queries.CreateHTTPBodyForm(ctx, gen.CreateHTTPBodyFormParams{
		ID:               form.ID,
		HttpID:           form.HttpID,
		FormKey:          form.FormKey,
		FormValue:        form.FormValue,
		Description:      form.Description,
		Enabled:          form.Enabled,
		ParentBodyFormID: form.ParentBodyFormID,
		IsDelta:          form.IsDelta,
		DeltaFormKey:     form.DeltaFormKey,
		DeltaFormValue:   form.DeltaFormValue,
		DeltaDescription: form.DeltaDescription,
		DeltaEnabled:     form.DeltaEnabled,
		Prev:             form.Prev,
		Next:             form.Next,
		CreatedAt:        form.CreatedAt,
		UpdatedAt:        form.UpdatedAt,
	})
}

func (hbfs HttpBodyFormService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	forms, err := hbfs.queries.GetHTTPBodyForms(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvert(forms, ConvertToModelHttpBodyForm), nil
}
