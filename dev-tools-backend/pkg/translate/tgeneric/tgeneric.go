package tgeneric

func MassConvert[T any, O any](item []T, convFunc func(T) O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = convFunc(v)
	}
	return arr
}

func MassConvertPtr[T any, O any](item []T, convFunc func(T) *O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = *convFunc(v)
	}
	return arr
}
