package reference

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"
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

type ReferenceKey struct {
	Kind  ReferenceKeyKind `protobuf:"varint,6661032,opt,name=kind,proto3,enum=reference.v1.ReferenceKeyKind" json:"kind,omitempty"`
	Group string           `protobuf:"bytes,49400938,opt,name=group,proto3,oneof" json:"group,omitempty"`
	Key   string           `protobuf:"bytes,7735364,opt,name=key,proto3,oneof" json:"key,omitempty"`
	Index int32            `protobuf:"varint,15866608,opt,name=index,proto3,oneof" json:"index,omitempty"`
}

type Reference struct {
	Kind     ReferenceKind `protobuf:"varint,9499794,opt,name=kind,proto3,enum=reference.v1.ReferenceKind" json:"kind,omitempty"`
	Key      ReferenceKey  `protobuf:"bytes,1233330,opt,name=key,proto3" json:"key,omitempty"`
	Map      []Reference   `protobuf:"bytes,15377576,rep,name=map,proto3" json:"map,omitempty"`           // Child map references
	Array    []Reference   `protobuf:"bytes,885261,rep,name=array,proto3" json:"array,omitempty"`         // Child array references
	Value    string        `protobuf:"bytes,24220210,opt,name=value,proto3,oneof" json:"value,omitempty"` // Primitive value as JSON string
	Variable []string      `protobuf:"bytes,24548959,rep,name=variable,proto3" json:"variable,omitempty"` // Environment names containing the variable
}

var (
	ErrNilMap   = errors.New("map is nil")
	ErrEmptyMap = errors.New("map is empty")
)

func ConvertMapToReference(m map[string]interface{}, key string) (Reference, error) {
	var ref Reference
	if m == nil {
		return ref, ErrNilMap
	}

	var subRefs []Reference
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
			valueRef := Reference{
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

	ref = Reference{
		Key: ReferenceKey{
			Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  key,
		},
		Kind: ReferenceKind_REFERENCE_KIND_MAP,
		Map:  subRefs,
	}

	return ref, nil
}

func ConvertPkgToRpc(ref Reference) *referencev1.Reference {
	return &referencev1.Reference{
		Kind: referencev1.ReferenceKind(ref.Kind),
		Key: &referencev1.ReferenceKey{
			Kind: referencev1.ReferenceKeyKind(ref.Key.Kind),
			Key:  &ref.Key.Key,
		},
		Value: &ref.Value,
		Map:   convertReferenceMap(ref.Map),
	}
}

func ConvertPkgKeyToRpc(ref ReferenceKey) *referencev1.ReferenceKey {
	return &referencev1.ReferenceKey{
		Kind:  referencev1.ReferenceKeyKind(ref.Kind),
		Group: &ref.Group,
		Key:   &ref.Key,
		Index: &ref.Index,
	}
}

func ConvertRpcToPkg(ref *referencev1.Reference) Reference {
	mapRefs := make([]Reference, len(ref.Map))
	arrayRefs := make([]Reference, len(ref.Array))
	value := ""

	for i, v := range ref.Map {
		mapRefs[i] = ConvertRpcToPkg(v)
	}

	for i, v := range ref.Array {
		arrayRefs[i] = ConvertRpcToPkg(v)
	}

	if ref.Value != nil {
		value = *ref.Value
	}

	return Reference{
		Kind:     ReferenceKind(ref.Kind),
		Key:      ConvertRpcKeyToPkgKey(ref.Key),
		Map:      mapRefs,
		Array:    arrayRefs,
		Value:    value,
		Variable: ref.Variable,
	}
}

func ConvertRpcKeyToPkgKey(ref *referencev1.ReferenceKey) ReferenceKey {
	if ref == nil {
		return ReferenceKey{}
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

	return ReferenceKey{
		Kind:  ReferenceKeyKind(ref.Kind),
		Group: group,
		Key:   key,
		Index: index,
	}
}

func convertReferenceMap(refs []Reference) []*referencev1.Reference {
	var result []*referencev1.Reference
	for _, ref := range refs {
		result = append(result, ConvertPkgToRpc(ref))
	}
	return result
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

func NewReferenceFromInterfaceWithKey(value any, key string) Reference {
	return NewReferenceFromInterface(value, ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: key})
}

func NewReferenceFromInterface(value any, key ReferenceKey) Reference {
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.Map:
		mapRefs := make([]Reference, 0, val.Len())
		keys := val.MapKeys()
		for _, mapKey := range keys {
			if mapKey.Kind() != reflect.String {
				continue
			}
			keyStr := mapKey.String()
			subKey := ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: keyStr}
			mapRefs = append(mapRefs, NewReferenceFromInterface(val.MapIndex(mapKey).Interface(), subKey))
		}
		return Reference{Key: key, Kind: ReferenceKind_REFERENCE_KIND_MAP, Map: mapRefs}
	case reflect.Slice, reflect.Array:
		arrayRefs := make([]Reference, val.Len())
		for i := range val.Len() {
			subKey := ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: int32(i)}
			arrayRefs[i] = NewReferenceFromInterface(val.Index(i).Interface(), subKey)
		}
		return Reference{Key: key, Kind: ReferenceKind_REFERENCE_KIND_ARRAY, Array: arrayRefs}
	case reflect.Struct:
		mapRefs := make([]Reference, 0, val.NumField())
		for i := range val.NumField() {
			field := val.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			subKey := ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: field.Name}
			fieldValue := NewReferenceFromInterface(val.Field(i).Interface(), subKey)
			mapRefs = append(mapRefs, fieldValue)
		}
		return Reference{Key: key, Kind: ReferenceKind_REFERENCE_KIND_MAP, Map: mapRefs}
	case reflect.String:
		return Reference{Key: key, Kind: ReferenceKind_REFERENCE_KIND_VALUE, Value: val.String()}
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64, reflect.Bool:
		return Reference{Key: key, Kind: ReferenceKind_REFERENCE_KIND_VALUE, Value: fmt.Sprintf("%v", val.Interface())}
	case reflect.Ptr:
		return NewReferenceFromInterface(val.Elem().Interface(), key)
	default:
		return Reference{}
	}
}
