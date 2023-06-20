package util

// Returns whether `value` is in `slice“
func Contains[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// Returns whether all elements of `values“ are in `slice`
func ContainsAll[T comparable](slice, values []T) bool {
	for _, v := range values {
		if !Contains(slice, v) {
			return false
		}
	}
	return true
}
