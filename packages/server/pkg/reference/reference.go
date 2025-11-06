package reference

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	referencev1 "the-dev-tools/spec/dist/buf/go/api/reference/v1"
)

type ReferenceKeyKind int32

const (
	ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED ReferenceKeyKind = 0
	ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP       ReferenceKeyKind = 1
	ReferenceKeyKind_REFERENCE_KEY_KIND_KEY         ReferenceKeyKind = 2
	ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX       ReferenceKeyKind = 3
	ReferenceKeyKind_REFERENCE_KEY_KIND_ANY         ReferenceKeyKind = 4
)

type ReferenceKind int32

const (
	ReferenceKind_REFERENCE_KIND_UNSPECIFIED ReferenceKind = 0
	ReferenceKind_REFERENCE_KIND_MAP         ReferenceKind = 1
	ReferenceKind_REFERENCE_KIND_ARRAY       ReferenceKind = 2
	ReferenceKind_REFERENCE_KIND_VALUE       ReferenceKind = 3
	ReferenceKind_REFERENCE_KIND_VARIABLE    ReferenceKind = 4
)

func referenceKindToProto(kind ReferenceKind) (referencev1.ReferenceKind, error) {
	switch kind {
	case ReferenceKind_REFERENCE_KIND_UNSPECIFIED:
		return referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED, nil
	case ReferenceKind_REFERENCE_KIND_MAP:
		return referencev1.ReferenceKind_REFERENCE_KIND_MAP, nil
	case ReferenceKind_REFERENCE_KIND_ARRAY:
		return referencev1.ReferenceKind_REFERENCE_KIND_ARRAY, nil
	case ReferenceKind_REFERENCE_KIND_VALUE:
		return referencev1.ReferenceKind_REFERENCE_KIND_VALUE, nil
	case ReferenceKind_REFERENCE_KIND_VARIABLE:
		return referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE, nil
	default:
		return referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED, fmt.Errorf("reference: unknown ReferenceKind %d", kind)
	}
}

func referenceKindFromProto(kind referencev1.ReferenceKind) (ReferenceKind, error) {
	switch kind {
	case referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED:
		return ReferenceKind_REFERENCE_KIND_UNSPECIFIED, nil
	case referencev1.ReferenceKind_REFERENCE_KIND_MAP:
		return ReferenceKind_REFERENCE_KIND_MAP, nil
	case referencev1.ReferenceKind_REFERENCE_KIND_ARRAY:
		return ReferenceKind_REFERENCE_KIND_ARRAY, nil
	case referencev1.ReferenceKind_REFERENCE_KIND_VALUE:
		return ReferenceKind_REFERENCE_KIND_VALUE, nil
	case referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE:
		return ReferenceKind_REFERENCE_KIND_VARIABLE, nil
	default:
		return ReferenceKind_REFERENCE_KIND_UNSPECIFIED, fmt.Errorf("reference: unknown referencev1.ReferenceKind %d", kind)
	}
}

func referenceKeyKindToProto(kind ReferenceKeyKind) (referencev1.ReferenceKeyKind, error) {
	switch kind {
	case ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED:
		return referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED, nil
	case ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP:
		return referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP, nil
	case ReferenceKeyKind_REFERENCE_KEY_KIND_KEY:
		return referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, nil
	case ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX:
		return referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, nil
	case ReferenceKeyKind_REFERENCE_KEY_KIND_ANY:
		return referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_ANY, nil
	default:
		return referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED, fmt.Errorf("reference: unknown ReferenceKeyKind %d", kind)
	}
}

func referenceKeyKindFromProto(kind referencev1.ReferenceKeyKind) (ReferenceKeyKind, error) {
	switch kind {
	case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED:
		return ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED, nil
	case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP:
		return ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP, nil
	case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY:
		return ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, nil
	case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX:
		return ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, nil
	case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_ANY:
		return ReferenceKeyKind_REFERENCE_KEY_KIND_ANY, nil
	default:
		return ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED, fmt.Errorf("reference: unknown referencev1.ReferenceKeyKind %d", kind)
	}
}

type ReferenceKey struct {
	Kind  ReferenceKeyKind `protobuf:"varint,6661032,opt,name=kind,proto3,enum=reference.v1.ReferenceKeyKind" json:"kind,omitempty"`
	Group string           `protobuf:"bytes,49400938,opt,name=group,proto3,oneof" json:"group,omitempty"`
	Key   string           `protobuf:"bytes,7735364,opt,name=key,proto3,oneof" json:"key,omitempty"`
	Index int32            `protobuf:"varint,15866608,opt,name=index,proto3,oneof" json:"index,omitempty"`
}

type ReferenceTreeItem struct {
	Kind     ReferenceKind       `protobuf:"varint,9499794,opt,name=kind,proto3,enum=reference.v1.ReferenceKind" json:"kind,omitempty"`
	Key      ReferenceKey        `protobuf:"bytes,1233330,opt,name=key,proto3" json:"key,omitempty"`
	Map      []ReferenceTreeItem `protobuf:"bytes,15377576,rep,name=map,proto3" json:"map,omitempty"`           // Child map references
	Array    []ReferenceTreeItem `protobuf:"bytes,885261,rep,name=array,proto3" json:"array,omitempty"`         // Child array references
	Value    string              `protobuf:"bytes,24220210,opt,name=value,proto3,oneof" json:"value,omitempty"` // Primitive value as JSON string
	Variable []string            `protobuf:"bytes,24548959,rep,name=variable,proto3" json:"variable,omitempty"` // Environment names containing the variable
}

var (
	ErrNilMap   = errors.New("map is nil")
	ErrEmptyMap = errors.New("map is empty")
)

func ConvertMapToReference(m map[string]interface{}, key string) (ReferenceTreeItem, error) {
	var ref ReferenceTreeItem
	if m == nil {
		return ref, ErrNilMap
	}

	var subRefs []ReferenceTreeItem
	for k, v := range m {
		vMap, ok := v.(map[string]interface{})
		key := ReferenceKey{
			Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  k,
		}
		if !ok {
			vStr, ok := v.(string)
			if !ok {
				vStr = fmt.Sprintf("%v", v)
			}
			valueRef := ReferenceTreeItem{
				Key:   key,
				Kind:  ReferenceKind_REFERENCE_KIND_VALUE,
				Value: vStr,
			}
			subRefs = append(subRefs, valueRef)
		} else {
			vRef, err := ConvertMapToReference(vMap, k)
			if err != nil {
				return ref, err
			}
			subRefs = append(subRefs, vRef)
		}
	}

	ref = ReferenceTreeItem{
		Key: ReferenceKey{
			Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  key,
		},
		Kind: ReferenceKind_REFERENCE_KIND_MAP,
		Map:  subRefs,
	}

	return ref, nil
}

func ConvertPkgToRpcTree(ref ReferenceTreeItem) (*referencev1.ReferenceTreeItem, error) {
	kind, err := referenceKindToProto(ref.Kind)
	if err != nil {
		return nil, fmt.Errorf("reference: convert pkg tree kind: %w", err)
	}

	key, err := ConvertPkgKeyToRpc(ref.Key)
	if err != nil {
		return nil, fmt.Errorf("reference: convert pkg tree key: %w", err)
	}

	mapRefs, err := convertReferenceMap(ref.Map)
	if err != nil {
		return nil, fmt.Errorf("reference: convert pkg tree map: %w", err)
	}

	arrayRefs, err := convertReferenceMap(ref.Array)
	if err != nil {
		return nil, fmt.Errorf("reference: convert pkg tree array: %w", err)
	}

	value := ref.Value

	return &referencev1.ReferenceTreeItem{
		Kind:     kind,
		Key:      key,
		Value:    &value,
		Map:      mapRefs,
		Array:    arrayRefs,
		Variable: ref.Variable,
	}, nil
}

func ConvertPkgKeyToRpc(ref ReferenceKey) (*referencev1.ReferenceKey, error) {
	kind, err := referenceKeyKindToProto(ref.Kind)
	if err != nil {
		return nil, fmt.Errorf("reference: convert pkg key kind: %w", err)
	}

	group := ref.Group
	key := ref.Key
	index := ref.Index

	return &referencev1.ReferenceKey{
		Kind:  kind,
		Group: &group,
		Key:   &key,
		Index: &index,
	}, nil
}

func ConvertRpcToPkg(ref *referencev1.ReferenceTreeItem) (ReferenceTreeItem, error) {
	if ref == nil {
		return ReferenceTreeItem{}, nil
	}

	mapRefs := make([]ReferenceTreeItem, len(ref.Map))
	for i, v := range ref.Map {
		converted, err := ConvertRpcToPkg(v)
		if err != nil {
			return ReferenceTreeItem{}, fmt.Errorf("reference: convert rpc map[%d]: %w", i, err)
		}
		mapRefs[i] = converted
	}

	arrayRefs := make([]ReferenceTreeItem, len(ref.Array))
	for i, v := range ref.Array {
		converted, err := ConvertRpcToPkg(v)
		if err != nil {
			return ReferenceTreeItem{}, fmt.Errorf("reference: convert rpc array[%d]: %w", i, err)
		}
		arrayRefs[i] = converted
	}

	key, err := ConvertRpcKeyToPkgKey(ref.Key)
	if err != nil {
		return ReferenceTreeItem{}, fmt.Errorf("reference: convert rpc key: %w", err)
	}

	kind, err := referenceKindFromProto(ref.Kind)
	if err != nil {
		return ReferenceTreeItem{}, fmt.Errorf("reference: convert rpc kind: %w", err)
	}

	value := ""
	if ref.Value != nil {
		value = *ref.Value
	}

	return ReferenceTreeItem{
		Kind:     kind,
		Key:      key,
		Map:      mapRefs,
		Array:    arrayRefs,
		Value:    value,
		Variable: ref.Variable,
	}, nil
}

func ConvertRpcKeyToPkgKey(ref *referencev1.ReferenceKey) (ReferenceKey, error) {
	if ref == nil {
		return ReferenceKey{}, nil
	}

	group := ""
	key := ""
	index := int32(0)
	if ref.Group != nil {
		group = *ref.Group
	}
	if ref.Key != nil {
		key = *ref.Key
	}
	if ref.Index != nil {
		index = *ref.Index
	}

	kind, err := referenceKeyKindFromProto(ref.Kind)
	if err != nil {
		return ReferenceKey{}, fmt.Errorf("reference: convert rpc key kind: %w", err)
	}

	return ReferenceKey{
		Kind:  kind,
		Group: group,
		Key:   key,
		Index: index,
	}, nil
}

func convertReferenceMap(refs []ReferenceTreeItem) ([]*referencev1.ReferenceTreeItem, error) {
	result := make([]*referencev1.ReferenceTreeItem, 0, len(refs))
	for _, ref := range refs {
		converted, err := ConvertPkgToRpcTree(ref)
		if err != nil {
			return nil, fmt.Errorf("reference: convert reference map item: %w", err)
		}
		result = append(result, converted)
	}
	return result, nil
}

func ConvertRefernceKeyArrayToStringPath(refKey []ReferenceKey) (string, error) {
	var path string

	for i, v := range refKey {
		switch v.Kind {
		case ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP:
			if v.Group == "" {
				return "", fmt.Errorf("group is nil")
			}
			if i != 0 {
				path += "."
			}
			path += v.Group
		case ReferenceKeyKind_REFERENCE_KEY_KIND_KEY:
			if v.Key == "" {
				return "", fmt.Errorf("key is nil")
			}
			if i != 0 {
				path += "."
			}
			path += v.Key
		case ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX:
			if i != 0 {
				return "", fmt.Errorf("cannot use index as first key")
			}
			path += fmt.Sprintf("[%d]", v.Index)
		default:
			// TODO: Add other types of reference keys here
			return "", fmt.Errorf("unknown reference key kind: %v", v.Kind)
		}
	}
	return path, nil
}

func ConvertStringPathToReferenceKeyArray(path string) ([]ReferenceKey, error) {
	if path == "" {
		return []ReferenceKey{}, nil
	}

	parts := strings.Split(path, ".")
	var refKeys []ReferenceKey

	for _, part := range parts {
		refKeys = append(refKeys, ReferenceKey{
			Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  part,
		})
	}
	return refKeys, nil
}

func NewReferenceFromInterfaceWithKey(value any, key string) ReferenceTreeItem {
	return NewReferenceFromInterface(value, ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: key})
}

func NewReferenceFromInterface(value any, key ReferenceKey) ReferenceTreeItem {
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.Map:
		mapRefs := make([]ReferenceTreeItem, 0, val.Len())
		keys := val.MapKeys()
		for _, mapKey := range keys {
			if mapKey.Kind() != reflect.String {
				continue
			}
			keyStr := mapKey.String()
			subKey := ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: keyStr}
			mapRefs = append(mapRefs, NewReferenceFromInterface(val.MapIndex(mapKey).Interface(), subKey))
		}
		return ReferenceTreeItem{Key: key, Kind: ReferenceKind_REFERENCE_KIND_MAP, Map: mapRefs}
	case reflect.Slice, reflect.Array:
		arrayRefs := make([]ReferenceTreeItem, val.Len())
		for i := range val.Len() {
			subKey := ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: int32(i)}
			arrayRefs[i] = NewReferenceFromInterface(val.Index(i).Interface(), subKey)
		}
		return ReferenceTreeItem{Key: key, Kind: ReferenceKind_REFERENCE_KIND_ARRAY, Array: arrayRefs}
	case reflect.Struct:
		mapRefs := make([]ReferenceTreeItem, 0, val.NumField())
		for i := range val.NumField() {
			field := val.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			subKey := ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: field.Name}
			fieldValue := NewReferenceFromInterface(val.Field(i).Interface(), subKey)
			mapRefs = append(mapRefs, fieldValue)
		}
		return ReferenceTreeItem{Key: key, Kind: ReferenceKind_REFERENCE_KIND_MAP, Map: mapRefs}
	case reflect.String:
		return ReferenceTreeItem{Key: key, Kind: ReferenceKind_REFERENCE_KIND_VALUE, Value: val.String()}
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64, reflect.Bool:
		return ReferenceTreeItem{Key: key, Kind: ReferenceKind_REFERENCE_KIND_VALUE, Value: fmt.Sprintf("%v", val.Interface())}
	case reflect.Ptr:
		return NewReferenceFromInterface(val.Elem().Interface(), key)
	case reflect.Int8:
		return ReferenceTreeItem{Key: key, Kind: ReferenceKind_REFERENCE_KIND_VALUE, Value: fmt.Sprintf("%v", val.Interface())}
	default:
		return ReferenceTreeItem{}
	}
}
