package expression

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"reflect"
	"strings"
	"the-dev-tools/server/pkg/flow/tracking"
	"the-dev-tools/server/pkg/varsystem"

	"github.com/expr-lang/expr"
)

type Env struct {
	varMap map[string]any
}

func NewEnv(varMap map[string]any) Env {
	return Env{
		varMap: varMap,
	}
}

// GetVarMap returns the internal varMap for debugging purposes
func (e Env) GetVarMap() map[string]any {
	return e.varMap
}

func NormalizeExpression(ctx context.Context, expressionString string, varsystem varsystem.VarMap) (string, error) {
	// trim spaces
	expressionString = strings.TrimSpace(expressionString)
	normalizedString, err := varsystem.ReplaceVars(expressionString)
	if err != nil {
		return expressionString, err
	}
	return normalizedString, nil

}

// convertStructToMapWithJSONTags recursively converts a struct to a map using JSON tags
func convertStructToMapWithJSONTags(v any) (any, error) {
	// Use JSON marshaling and unmarshaling to handle nested structs with JSON tags
	jsonData, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result any
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func NewEnvFromStruct(s any) (Env, error) {
	varMap := make(map[string]any)
	val := reflect.ValueOf(s)

	if val.Kind() != reflect.Struct {
		return Env{}, fmt.Errorf("input is not a struct, got %T", s)
	}

	typ := reflect.TypeOf(s)
	for i := range val.NumField() {
		fieldValue := val.Field(i)
		field := typ.Field(i)

		// Use JSON tag if available, otherwise use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			// Handle JSON tag options like "fieldname,omitempty"
			jsonFieldName := jsonTag
			if commaIndex := strings.Index(jsonTag, ","); commaIndex != -1 {
				jsonFieldName = jsonTag[:commaIndex]
			}
			if jsonFieldName != "" && jsonFieldName != "-" {
				fieldName = jsonFieldName
			}
		}

		// Convert the field value to use JSON tag names recursively
		convertedValue, err := convertStructToMapWithJSONTags(fieldValue.Interface())
		if err != nil {
			return Env{}, err
		}

		varMap[fieldName] = convertedValue
	}

	return NewEnv(varMap), nil
}

func ExpressionEvaluteAsBool(ctx context.Context, env Env, expressionString string) (bool, error) {
	program, err := expr.Compile(expressionString, expr.AsBool(), expr.Env(env.varMap))
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, env.varMap)
	if err != nil {
		return false, err
	}

	ok := output.(bool)
	return ok, nil
}

func ExpressionEvaluteAsArray(ctx context.Context, env Env, expressionString string) ([]any, error) {
	program, err := expr.Compile(expressionString, expr.AsAny(), expr.Env(env.varMap))
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env.varMap) // Pass the map directly
	if err != nil {
		return nil, err
	}

	// expr.Run can return []interface{} for arrays. Convert it to []any.
	if outputSlice, ok := output.([]any); ok {
		return outputSlice, nil
	}

	// If it's not []interface{}, check if it's already []any (less common for expr output)
	if outputAnySlice, ok := output.([]any); ok {
		return outputAnySlice, nil
	}

	// If it's neither, it's not an array
	return nil, fmt.Errorf("expected array, but got %T", output)
}

// ExpressionEvaluateAsIter evaluates the expression and returns an iterator sequence
// (iter.Seq[any] for slices, iter.Seq2[string, any] for maps) if the result is iterable.
// Otherwise, it returns an error.
func ExpressionEvaluateAsIter(ctx context.Context, env Env, expressionString string) (any, error) {
	program, err := expr.Compile(expressionString, expr.AsAny(), expr.Env(env.varMap))
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env.varMap)
	if err != nil {
		return nil, err
	}

	// Check if the result is an iterable type (map or slice/array)
	val := reflect.ValueOf(output)
	switch val.Kind() {
	case reflect.Map:
		// Handle map iteration
		if val.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings for iteration, got %s", val.Type().Key().Kind())
		}
		seq := func(yield func(string, any) bool) {
			for _, key := range val.MapKeys() {
				k := key.String()
				v := val.MapIndex(key).Interface()
				if !yield(k, v) {
					return
				}
			}
		}
		return iter.Seq2[string, any](seq), nil

	case reflect.Slice, reflect.Array:
		// Handle slice/array iteration
		seq := func(yield func(any) bool) {
			for i := range val.Len() {
				item := val.Index(i).Interface()
				if !yield(item) {
					return
				}
			}
		}
		return iter.Seq[any](seq), nil

	default:
		return nil, fmt.Errorf("expected iterable (map or slice/array), but got %T", output)
	}
}

// ExpressionEvaluteAsBoolWithTracking evaluates a boolean expression with variable access tracking
func ExpressionEvaluteAsBoolWithTracking(ctx context.Context, env Env, expressionString string, tracker *tracking.VariableTracker) (bool, error) {
	if tracker == nil {
		// If no tracker provided, use regular function
		return ExpressionEvaluteAsBool(ctx, env, expressionString)
	}

	trackedEnv := tracking.NewTrackingEnv(env.varMap, tracker)

	// Track all variables as potentially accessed since we can't track individual access
	trackedEnv.TrackAllVariables()

	program, err := expr.Compile(expressionString, expr.AsBool(), expr.Env(trackedEnv.GetMap()))
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, trackedEnv.GetMap())
	if err != nil {
		return false, err
	}

	ok := output.(bool)
	return ok, nil
}

// ExpressionEvaluteAsArrayWithTracking evaluates an array expression with variable access tracking
func ExpressionEvaluteAsArrayWithTracking(ctx context.Context, env Env, expressionString string, tracker *tracking.VariableTracker) ([]any, error) {
	if tracker == nil {
		// If no tracker provided, use regular function
		return ExpressionEvaluteAsArray(ctx, env, expressionString)
	}

	trackedEnv := tracking.NewTrackingEnv(env.varMap, tracker)

	// Track all variables as potentially accessed since we can't track individual access
	trackedEnv.TrackAllVariables()

	program, err := expr.Compile(expressionString, expr.AsAny(), expr.Env(trackedEnv.GetMap()))
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, trackedEnv.GetMap())
	if err != nil {
		return nil, err
	}

	// expr.Run can return []interface{} for arrays. Convert it to []any.
	if outputSlice, ok := output.([]any); ok {
		return outputSlice, nil
	}

	// If it's not []interface{}, check if it's already []any (less common for expr output)
	if outputAnySlice, ok := output.([]any); ok {
		return outputAnySlice, nil
	}

	// If it's neither, it's not an array
	return nil, fmt.Errorf("expected array, but got %T", output)
}

// ExpressionEvaluateAsIterWithTracking evaluates an iterable expression with variable access tracking
func ExpressionEvaluateAsIterWithTracking(ctx context.Context, env Env, expressionString string, tracker *tracking.VariableTracker) (any, error) {
	if tracker == nil {
		// If no tracker provided, use regular function
		return ExpressionEvaluateAsIter(ctx, env, expressionString)
	}

	trackedEnv := tracking.NewTrackingEnv(env.varMap, tracker)

	// Track all variables as potentially accessed since we can't track individual access
	trackedEnv.TrackAllVariables()

	program, err := expr.Compile(expressionString, expr.AsAny(), expr.Env(trackedEnv.GetMap()))
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, trackedEnv.GetMap())
	if err != nil {
		return nil, err
	}

	// Check if the result is an iterable type (map or slice/array)
	val := reflect.ValueOf(output)
	switch val.Kind() {
	case reflect.Map:
		// Handle map iteration
		if val.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings for iteration, got %s", val.Type().Key().Kind())
		}
		seq := func(yield func(string, any) bool) {
			for _, key := range val.MapKeys() {
				k := key.String()
				v := val.MapIndex(key).Interface()
				if !yield(k, v) {
					return
				}
			}
		}
		return iter.Seq2[string, any](seq), nil

	case reflect.Slice, reflect.Array:
		// Handle slice/array iteration
		seq := func(yield func(any) bool) {
			for i := range val.Len() {
				item := val.Index(i).Interface()
				if !yield(item) {
					return
				}
			}
		}
		return iter.Seq[any](seq), nil

	default:
		return nil, fmt.Errorf("expected iterable (map or slice/array), but got %T", output)
	}
}
