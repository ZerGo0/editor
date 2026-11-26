//go:generate go run ./gen

package vscode

type Entry struct {
	Key     string
	Command string
	When    string
	Args    string
}
