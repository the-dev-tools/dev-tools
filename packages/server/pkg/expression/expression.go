//nolint:revive // exported
package expression

import (
	"context"
	"encoding"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"strings"
	"sync"
	"the-dev-tools/server/pkg/errmap"
	"the-dev-tools/server/pkg/flow/tracking"
	"the-dev-tools/server/pkg/varsystem"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/vm"
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
var (
	jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	structFieldCache  sync.Map // map[reflect.Type][]structFieldInfo
)

type structFieldInfo struct {
	name      string
	omitEmpty bool
	index     []int
}

type compileMode uint8

const (
	compileModeAny compileMode = iota
	compileModeBool
)

type expressionPhase uint8

const (
	expressionPhaseCompile expressionPhase = iota
	expressionPhaseRun
)

type programCacheKey struct {
	expression string
	mode       compileMode
}

var (
	programCache    sync.Map // map[programCacheKey]*vm.Program
	emptyCompileEnv = map[string]any{}
)

// convertStructToMapWithJSONTags recursively converts a value to a map/array primitive structure
// while respecting json struct tags. It mirrors the shape that encoding/json would produce when
// unmarshalling into map[string]any without paying the serialization cost for every field.
func convertStructToMapWithJSONTags(v any) (any, error) {
	return convertValue(reflect.ValueOf(v))
}

func convertValue(val reflect.Value) (any, error) {
	if !val.IsValid() {
		return nil, nil
	}

	if val.Kind() == reflect.Interface || val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return nil, nil
		}
		return convertValue(val.Elem())
	}

	switch val.Kind() {
	case reflect.Struct:
		return convertStruct(val)
	case reflect.Map:
		return convertMap(val)
	case reflect.Slice, reflect.Array:
		return convertSlice(val)
	case reflect.String:
		return val.String(), nil
	case reflect.Bool:
		return val.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(val.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(val.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return val.Convert(reflect.TypeOf(float64(0))).Interface(), nil
	case reflect.Complex64, reflect.Complex128:
		// encoding/json marshals complex numbers as maps with real/imag parts; fall back to JSON.
		return marshalViaJSON(val)
	default:
		// For other types (e.g., custom types implementing json.Marshaler) we fall back to JSON
		// to preserve their custom encoding behaviour.
		return marshalViaJSON(val)
	}
}

func convertStruct(val reflect.Value) (any, error) {
	// Honour custom JSON/text marshalers.
	typ := val.Type()
	if implementsJSONMarshaler(typ) || implementsTextMarshaler(typ) {
		return marshalViaJSON(val)
	}

	fields := getStructFields(typ)
	result := make(map[string]any, len(fields))
	for _, fieldInfo := range fields {
		fieldVal := val.FieldByIndex(fieldInfo.index)
		if fieldInfo.omitEmpty && isZeroValue(fieldVal) {
			continue
		}
		converted, err := convertValue(fieldVal)
		if err != nil {
			return nil, err
		}
		result[fieldInfo.name] = converted
	}
	return result, nil
}

func convertMap(val reflect.Value) (any, error) {
	if val.IsNil() {
		return nil, nil
	}
	if val.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("map keys must be strings for JSON conversion, got %s", val.Type().Key())
	}
	result := make(map[string]any, val.Len())
	iter := val.MapRange()
	for iter.Next() {
		key := iter.Key().String()
		converted, err := convertValue(iter.Value())
		if err != nil {
			return nil, err
		}
		result[key] = converted
	}
	return result, nil
}

func convertSlice(val reflect.Value) (any, error) {
	if val.Kind() == reflect.Slice && val.IsNil() {
		return nil, nil
	}
	if val.Kind() == reflect.Slice && val.Type().Elem().Kind() == reflect.Uint8 {
		// Match encoding/json which converts []byte to base64 string
		bytes := make([]byte, val.Len())
		reflect.Copy(reflect.ValueOf(bytes), val)
		return base64.StdEncoding.EncodeToString(bytes), nil
	}
	result := make([]any, val.Len())
	for i := 0; i < val.Len(); i++ {
		converted, err := convertValue(val.Index(i))
		if err != nil {
			return nil, err
		}
		result[i] = converted
	}
	return result, nil
}

func marshalViaJSON(val reflect.Value) (any, error) {
	if !val.CanInterface() {
		return nil, fmt.Errorf("cannot interface value of type %s", val.Type())
	}
	data, err := json.Marshal(val.Interface())
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseJSONTag(tag string, defaultName string) (name string, omitEmpty bool, skip bool) {
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		return defaultName, false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = defaultName
	}
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func getStructFields(t reflect.Type) []structFieldInfo {
	if cached, ok := structFieldCache.Load(t); ok {
		return cached.([]structFieldInfo)
	}

	fields := make([]structFieldInfo, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		name, omitEmpty, skip := parseJSONTag(field.Tag.Get("json"), field.Name)
		if skip {
			continue
		}
		fields = append(fields, structFieldInfo{
			name:      name,
			omitEmpty: omitEmpty,
			index:     field.Index,
		})
	}
	structFieldCache.Store(t, fields)
	return fields
}

func compileProgram(expression string, mode compileMode, env map[string]any) (*vm.Program, error) {
	key := programCacheKey{expression: expression, mode: mode}
	if cached, ok := programCache.Load(key); ok {
		return cached.(*vm.Program), nil
	}

	compileEnv := env
	if compileEnv == nil {
		compileEnv = emptyCompileEnv
	}

	options := []expr.Option{expr.Env(compileEnv)}
	switch mode {
	case compileModeBool:
		options = append(options, expr.AsBool())
	default:
		options = append(options, expr.AsAny())
	}

	program, err := expr.Compile(expression, options...)
	if err != nil {
		return nil, wrapExpressionError(expression, expressionPhaseCompile, err)
	}
	programCache.Store(key, program)
	return program, nil
}

func wrapExpressionError(expression string, phase expressionPhase, err error) error {
	if err == nil {
		return nil
	}

	code := errmap.CodeExpressionRuntime
	phaseVerb := "evaluating"
	if phase == expressionPhaseCompile {
		code = errmap.CodeExpressionSyntax
		phaseVerb = "parsing"
	}

	var fileErr *file.Error
	if errors.As(err, &fileErr) {
		line := fileErr.Line
		column := fileErr.Column + 1
		location := ""
		if line > 0 {
			location = fmt.Sprintf(" at line %d", line)
			if column > 0 {
				location += fmt.Sprintf(" column %d", column)
			}
		}

		message := fmt.Sprintf("error %s expression%s: %s", phaseVerb, location, fileErr.Message)
		if snippet := fileErr.Snippet; snippet != "" {
			message += snippet
		}

		return errmap.New(code, message, err)
	}

	message := fmt.Sprintf("error %s expression: %v", phaseVerb, err)
	return errmap.New(code, message, err)
}

func isZeroValue(val reflect.Value) bool {
	// reflect.Value.IsZero panics for invalid values, but we've handled invalid earlier.
	return val.IsZero()
}

func implementsJSONMarshaler(t reflect.Type) bool {
	if t.Implements(jsonMarshalerType) {
		return true
	}
	if t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(jsonMarshalerType) {
		return true
	}
	return false
}

func implementsTextMarshaler(t reflect.Type) bool {
	if t.Implements(textMarshalerType) {
		return true
	}
	if t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(textMarshalerType) {
		return true
	}
	return false
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
	program, err := compileProgram(expressionString, compileModeBool, env.varMap)
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, env.varMap)
	if err != nil {
		return false, wrapExpressionError(expressionString, expressionPhaseRun, err)
	}

	ok := output.(bool)
	return ok, nil
}

func ExpressionEvaluteAsArray(ctx context.Context, env Env, expressionString string) ([]any, error) {
	program, err := compileProgram(expressionString, compileModeAny, env.varMap)
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env.varMap) // Pass the map directly
	if err != nil {
		return nil, wrapExpressionError(expressionString, expressionPhaseRun, err)
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
	program, err := compileProgram(expressionString, compileModeAny, env.varMap)
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env.varMap)
	if err != nil {
		return nil, wrapExpressionError(expressionString, expressionPhaseRun, err)
	}

	if output == nil {
		return iter.Seq[any](func(func(any) bool) {}), nil
	}

	if str, ok := output.(string); ok {
		if strings.TrimSpace(str) == "" {
			return iter.Seq[any](func(func(any) bool) {}), nil
		}
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

	program, err := compileProgram(expressionString, compileModeBool, trackedEnv.GetMap())
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, trackedEnv.GetMap())
	if err != nil {
		return false, wrapExpressionError(expressionString, expressionPhaseRun, err)
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

	program, err := compileProgram(expressionString, compileModeAny, trackedEnv.GetMap())
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, trackedEnv.GetMap())
	if err != nil {
		return nil, wrapExpressionError(expressionString, expressionPhaseRun, err)
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

	program, err := compileProgram(expressionString, compileModeAny, trackedEnv.GetMap())
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, trackedEnv.GetMap())
	if err != nil {
		return nil, wrapExpressionError(expressionString, expressionPhaseRun, err)
	}

	if output == nil {
		return iter.Seq[any](func(func(any) bool) {}), nil
	}

	if str, ok := output.(string); ok {
		if strings.TrimSpace(str) == "" {
			return iter.Seq[any](func(func(any) bool) {}), nil
		}
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
