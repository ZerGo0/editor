package testutil

func Map[T, U any](items []T, f func(T) U) []U {
	out := make([]U, len(items))
	for i, it := range items {
		out[i] = f(it)
	}
	return out
}
