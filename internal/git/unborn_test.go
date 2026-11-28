package git

import (
	"context"
	"testing"
)

func TestLogOfUnbornHeadIsEmpty(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	commits, err := repo.Log(context.Background(), LogOptions{Revs: []string{"HEAD"}, Max: 10})
	if err != nil || len(commits) != 0 {
		t.Fatalf("Log = %v, %v; want no commits and no error", commits, err)
	}
	if _, err := repo.Log(context.Background(), LogOptions{Revs: []string{"no-such-ref"}}); err == nil {
		t.Fatal("Log of a missing ref returned no error")
	}
}
