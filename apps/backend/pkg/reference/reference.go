package reference

import (
	"errors"
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
				return ref, errors.New("value is not a string")
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

func ConvertRpc(ref Reference) *referencev1.Reference {
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

func convertReferenceMap(refs []Reference) []*referencev1.Reference {
	var result []*referencev1.Reference
	for _, ref := range refs {
		result = append(result, ConvertRpc(ref))
	}
	return result
}
