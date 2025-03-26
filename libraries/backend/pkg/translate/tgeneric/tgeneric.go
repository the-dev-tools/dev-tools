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

func MapToSlice[T any, K comparable](item map[K]T) []T {
	arr := make([]T, 0, len(item))
	for _, v := range item {
		arr = append(arr, v)
	}
	return arr
}

func ReplaceRootWithSub[T comparable](rootError, subError, got T) T {
	if got == rootError {
		return subError
	}
	return got
}

const thresholdSwitchRemove = 100

func RemoveElement[T comparable](arr []T, v T) []T {
	if len(arr) < thresholdSwitchRemove {
		return RemoveElementSmall(arr, v)
	} else {
		return RemoveElementBig(arr, v)
	}
}

func RemoveElementSmall[T comparable](arr []T, v T) []T {
	var result []T
	for _, e := range arr {
		if e != v {
			result = append(result, e)
		}
	}
	return result
}

func RemoveElementBig[T comparable](arr []T, v T) []T {
	a := make(map[T]struct{})
	for _, v := range arr {
		a[v] = struct{}{}
	}

	delete(a, v)

	result := make([]T, 0, len(a))
	for k := range a {
		result = append(result, k)
	}

	return result
}
