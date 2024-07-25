package convert_test

import (
	"devtools-nodes/pkg/convert"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/nodes/api"
	"testing"
)

func TestConvertStructToMsg(t *testing.T) {
	t.Run("Test unsupported type", func(t *testing.T) {
		msg, err := convert.ConvertStructToMsg(nil)
		if err == nil {
			t.Error("Expected got error, got", err)
		}

		if msg != nil {
			t.Error("Expected nil message, got", msg)
		}

		msg, err = convert.ConvertStructToMsg(&struct{}{})
		if err == nil {
			t.Error("Expected got error, got", err)
		}

		if msg != nil {
			t.Error("Expected nil message, got", msg)
		}

		msg, err = convert.ConvertStructToMsg("test")
		if err == nil {
			t.Error("Expected got error, got", err)
		}

		if msg != nil {
			t.Error("Expected nil message, got", msg)
		}

		msg, err = convert.ConvertStructToMsg(1)
		if err == nil {
			t.Error("Expected got error, got", err)
		}

		if msg != nil {
			t.Error("Expected nil message, got", msg)
		}

		msg, err = convert.ConvertStructToMsg(1.1)
		if err == nil {
			t.Error("Expected got error, got", err)
		}

		if msg != nil {
			t.Error("Expected nil message, got", msg)
		}
	})

	t.Run("Test supported type", func(t *testing.T) {
		msg, err := convert.ConvertStructToMsg(&api.RestApiData{})
		if err != nil {
			t.Error("Expected nil error, got", err)
		}

		if msg == nil {
			t.Error("Expected message, got nil")
		}

		msg, err = convert.ConvertStructToMsg(&mnodedata.LoopRemoteData{})
		if err != nil {
			t.Error("Expected nil error, got", err)
		}

		if msg == nil {
			t.Error("Expected message, got nil")
		}
	})
}
