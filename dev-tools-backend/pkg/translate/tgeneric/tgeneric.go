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

func MassConvertWithErr[T any, O any](item []T, convFunc func(T) (O, error)) ([]O, error) {
	arr := make([]O, len(item))
	var err error
	for i, v := range item {
		arr[i], err = convFunc(v)
		if err != nil {
			return nil, err
		}
	}
	return arr, nil
}
