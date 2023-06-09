package literal

func ToPointer[T any](l T) *T {
	return &l
}
