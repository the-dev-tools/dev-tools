package shttp

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHttpBodyFormFound = errors.New("no HTTP body form found")

type HttpBodyFormService struct {
	queries *gen.Queries
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

func ConvertToDBHttpBodyForm(form mhttp.HTTPBodyForm) gen.CreateHTTPBodyFormParams {
	return gen.CreateHTTPBodyFormParams{
		ID:                   form.ID,
		HttpID:               form.HttpID,
		Key:                  form.FormKey,
		Value:                form.FormValue,
		Description:          form.Description,
		Enabled:              form.Enabled,
		Order:                0, // Default order since model doesn't have this field
		ParentHttpBodyFormID: idWrapToBytes(form.ParentBodyFormID),
		IsDelta:              form.IsDelta,
		DeltaKey:             stringToNull(form.DeltaFormKey),
		DeltaValue:           stringToNull(form.DeltaFormValue),
		DeltaDescription:     form.DeltaDescription,
		DeltaEnabled:         form.DeltaEnabled,
		CreatedAt:            form.CreatedAt,
		UpdatedAt:            form.UpdatedAt,
	}
}

func ConvertToModelHttpBodyForm(form gen.GetHTTPBodyFormsRow) mhttp.HTTPBodyForm {
	return mhttp.HTTPBodyForm{
		ID:               form.ID,
		HttpID:           form.HttpID,
		FormKey:          form.Key,
		FormValue:        form.Value,
		Description:      form.Description,
		Enabled:          form.Enabled,
		ParentBodyFormID: bytesToIDWrap(form.ParentHttpBodyFormID),
		IsDelta:          form.IsDelta,
		DeltaFormKey:     nullToString(form.DeltaKey),
		DeltaFormValue:   nullToString(form.DeltaValue),
		DeltaDescription: form.DeltaDescription,
		DeltaEnabled:     form.DeltaEnabled,
		Prev:             nil, // No equivalent in database schema
		Next:             nil, // No equivalent in database schema
		CreatedAt:        form.CreatedAt,
		UpdatedAt:        form.UpdatedAt,
	}
}

func ConvertToModelHttpBodyFormFromIDs(form gen.GetHTTPBodyFormsByIDsRow) mhttp.HTTPBodyForm {
	return mhttp.HTTPBodyForm{
		ID:               form.ID,
		HttpID:           form.HttpID,
		FormKey:          form.Key,
		FormValue:        form.Value,
		Description:      form.Description,
		Enabled:          form.Enabled,
		ParentBodyFormID: bytesToIDWrap(form.ParentHttpBodyFormID),
		IsDelta:          form.IsDelta,
		DeltaFormKey:     nullToString(form.DeltaKey),
		DeltaFormValue:   nullToString(form.DeltaValue),
		DeltaDescription: form.DeltaDescription,
		DeltaEnabled:     form.DeltaEnabled,
		Prev:             nil, // No equivalent in database schema
		Next:             nil, // No equivalent in database schema
		CreatedAt:        form.CreatedAt,
		UpdatedAt:        form.UpdatedAt,
	}
}

func (hbfs HttpBodyFormService) Create(ctx context.Context, form mhttp.HTTPBodyForm) error {
	dbForm := ConvertToDBHttpBodyForm(form)
	return hbfs.queries.CreateHTTPBodyForm(ctx, dbForm)
}

func (hbfs HttpBodyFormService) CreateBulk(ctx context.Context, forms []mhttp.HTTPBodyForm) error {
	for _, form := range forms {
		if err := hbfs.Create(ctx, form); err != nil {
			return err
		}
	}
	return nil
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

func (hbfs HttpBodyFormService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	forms, err := hbfs.queries.GetHTTPBodyFormsByIDs(ctx, ids)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvert(forms, ConvertToModelHttpBodyFormFromIDs), nil
}

func (hbfs HttpBodyFormService) Update(ctx context.Context, form mhttp.HTTPBodyForm) error {
	dbForm := ConvertToDBHttpBodyForm(form)
	return hbfs.queries.UpdateHTTPBodyForm(ctx, gen.UpdateHTTPBodyFormParams{
		Key:         dbForm.Key,
		Value:       dbForm.Value,
		Description: dbForm.Description,
		Enabled:     dbForm.Enabled,
		ID:          dbForm.ID,
	})
}

func (hbfs HttpBodyFormService) UpdateDelta(ctx context.Context, form mhttp.HTTPBodyForm) error {
	return hbfs.queries.UpdateHTTPBodyFormDelta(ctx, gen.UpdateHTTPBodyFormDeltaParams{
		ID:               form.ID,
		DeltaKey:         stringToNull(form.DeltaFormKey),
		DeltaValue:       stringToNull(form.DeltaFormValue),
		DeltaDescription: form.DeltaDescription,
		DeltaEnabled:     form.DeltaEnabled,
	})
}

func (hbfs HttpBodyFormService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return hbfs.queries.DeleteHTTPBodyForm(ctx, id)
}
