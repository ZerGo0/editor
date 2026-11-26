package action

func Hint(label, keys string) string {
	if keys == "" {
		return label
	}
	return label + " (" + keys + ")"
}

func keyOf(id, label string) string {
	if id != "" {
		return id
	}
	return label
}
