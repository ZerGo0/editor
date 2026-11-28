package plural

import "strconv"

func Count(n int, one, many string) string {
	return strconv.Itoa(n) + " " + Word(n, one, many)
}

func Word(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
