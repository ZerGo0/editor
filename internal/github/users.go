package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	assignableUsersField  = "assignableUsers"
	mentionableUsersField = "mentionableUsers"
	userPageSize          = 100

	linkedBranchTargetQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){defaultBranchRef{target{oid}} issue(number:$number){id}}}`

	createLinkedBranchMutation = `mutation($issue:ID!,$oid:GitObjectID!,$name:String!){createLinkedBranch(input:{issueId:$issue,oid:$oid,name:$name}){linkedBranch{id ref{name}}}}`
)

type User struct {
	Login     string
	Name      string
	AvatarURL string
}

type UserPage struct {
	Users []User
	Next  string
}

type LinkedBranch struct {
	ID   string
	Name string
}

func (c *Client) Viewer(ctx context.Context) (User, error) {
	var raw struct {
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/user", nil, &raw); err != nil {
		return User{}, err
	}
	if raw.Login == "" {
		return User{}, errors.New("github: user response lacks login")
	}
	return User{Login: raw.Login, Name: raw.Name, AvatarURL: raw.AvatarURL}, nil
}

func (c *Client) AssignableUsers(ctx context.Context, repo Remote, after string) (UserPage, error) {
	return c.userPage(ctx, repo, assignableUsersField, after)
}

func (c *Client) MentionableUsers(ctx context.Context, repo Remote, after string) (UserPage, error) {
	return c.userPage(ctx, repo, mentionableUsersField, after)
}

func (c *Client) userPage(ctx context.Context, repo Remote, field, after string) (UserPage, error) {
	if _, err := repoURL(repo); err != nil {
		return UserPage{}, err
	}
	query := fmt.Sprintf(`query($owner:String!,$name:String!,$first:Int!,$after:String){repository(owner:$owner,name:$name){%s(first:$first,after:$after){pageInfo{hasNextPage endCursor} nodes{login name avatarUrl}}}}`, field)
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "first": userPageSize, "after": nil}
	if after != "" {
		vars["after"] = after
	}
	var data struct {
		Repository map[string]*struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []struct {
				Login     string `json:"login"`
				Name      string `json:"name"`
				AvatarURL string `json:"avatarUrl"`
			} `json:"nodes"`
		} `json:"repository"`
	}
	if err := c.graphql(ctx, query, vars, &data); err != nil {
		return UserPage{}, err
	}
	page := data.Repository[field]
	if page == nil {
		return UserPage{}, fmt.Errorf("github: repository %s/%s not found", repo.Owner, repo.Name)
	}
	out := UserPage{Users: make([]User, 0, len(page.Nodes))}
	for _, n := range page.Nodes {
		if n.Login == "" {
			return UserPage{}, errors.New("github: user entry lacks login")
		}
		out.Users = append(out.Users, User{Login: n.Login, Name: n.Name, AvatarURL: n.AvatarURL})
	}
	if page.PageInfo.HasNextPage {
		if page.PageInfo.EndCursor == "" {
			return UserPage{}, errors.New("github: graphql page lacks end cursor")
		}
		out.Next = page.PageInfo.EndCursor
	}
	return out, nil
}

func (c *Client) CreateLinkedBranch(ctx context.Context, repo Remote, number int, name, baseSHA string) (LinkedBranch, error) {
	if _, err := repoURL(repo); err != nil {
		return LinkedBranch{}, err
	}
	if err := checkIssueNumber(number); err != nil {
		return LinkedBranch{}, err
	}
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, " \t\n") {
		return LinkedBranch{}, fmt.Errorf("github: invalid branch name %q", name)
	}
	var target struct {
		Repository *struct {
			DefaultBranchRef *struct {
				Target struct {
					OID string `json:"oid"`
				} `json:"target"`
			} `json:"defaultBranchRef"`
			Issue *struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"repository"`
	}
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number}
	if err := c.graphql(ctx, linkedBranchTargetQuery, vars, &target); err != nil {
		return LinkedBranch{}, err
	}
	if target.Repository == nil || target.Repository.Issue == nil || target.Repository.Issue.ID == "" {
		return LinkedBranch{}, fmt.Errorf("github: issue %d not found", number)
	}
	oid := baseSHA
	if oid == "" && target.Repository.DefaultBranchRef != nil {
		oid = target.Repository.DefaultBranchRef.Target.OID
	}
	if oid == "" {
		return LinkedBranch{}, errors.New("github: no base commit for the linked branch")
	}
	var data struct {
		Payload *struct {
			LinkedBranch *struct {
				ID  string `json:"id"`
				Ref *struct {
					Name string `json:"name"`
				} `json:"ref"`
			} `json:"linkedBranch"`
		} `json:"createLinkedBranch"`
	}
	mutation := map[string]any{"issue": target.Repository.Issue.ID, "oid": oid, "name": name}
	if err := c.graphql(ctx, createLinkedBranchMutation, mutation, &data); err != nil {
		return LinkedBranch{}, err
	}
	if data.Payload == nil || data.Payload.LinkedBranch == nil || data.Payload.LinkedBranch.Ref == nil {
		return LinkedBranch{}, errors.New("github: linked branch response lacks branch")
	}
	return LinkedBranch{ID: data.Payload.LinkedBranch.ID, Name: data.Payload.LinkedBranch.Ref.Name}, nil
}
