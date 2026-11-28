package github

import "strings"

var override struct {
	baseURL string
	token   string
}

func SetOverride(baseURL, token string) {
	override.baseURL = strings.TrimRight(baseURL, "/")
	override.token = token
}

func OverrideURLs() (rest, graphql string, ok bool) {
	if override.baseURL == "" {
		return "", "", false
	}
	return override.baseURL, override.baseURL + graphQLSuffix, true
}
