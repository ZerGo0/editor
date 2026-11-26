package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "gitpkg-home")
	if err != nil {
		panic(err)
	}
	env := map[string]string{
		"HOME":                home,
		"XDG_CONFIG_HOME":     home,
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Ada Lovelace",
		"GIT_AUTHOR_EMAIL":    "ada@example.com",
		"GIT_COMMITTER_NAME":  "Ada Lovelace",
		"GIT_COMMITTER_EMAIL": "ada@example.com",
		"GIT_CONFIG_COUNT":    "3",
		"GIT_CONFIG_KEY_0":    "init.defaultBranch",
		"GIT_CONFIG_VALUE_0":  "main",
		"GIT_CONFIG_KEY_1":    "protocol.file.allow",
		"GIT_CONFIG_VALUE_1":  "always",
		"GIT_CONFIG_KEY_2":    "commit.gpgsign",
		"GIT_CONFIG_VALUE_2":  "false",
	}
	for k, v := range env {
		os.Setenv(k, v)
	}
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

func rawGit(t testing.TB, dir string, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t testing.TB) *Repo {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rawGit(t, dir, "", "init", "--quiet")
	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func newBareRemote(t testing.TB) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rawGit(t, dir, "", "init", "--quiet", "--bare")
	return dir
}

func put(t testing.TB, repo *Repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func slurp(t testing.TB, repo *Repo, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo.Root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func removePath(repo *Repo, rel string) error {
	return os.Remove(filepath.Join(repo.Root, filepath.FromSlash(rel)))
}

func exists(repo *Repo, rel string) bool {
	_, err := os.Stat(filepath.Join(repo.Root, filepath.FromSlash(rel)))
	return err == nil
}

func g(t testing.TB, repo *Repo, args ...string) string {
	t.Helper()
	return rawGit(t, repo.Root, "", args...)
}

func commitAll(t testing.TB, repo *Repo, msg string) string {
	t.Helper()
	g(t, repo, "add", "-A")
	g(t, repo, "commit", "--quiet", "-m", msg)
	return g(t, repo, "rev-parse", "HEAD")
}

func commitFile(t testing.TB, repo *Repo, rel, content, msg string) string {
	t.Helper()
	put(t, repo, rel, content)
	return commitAll(t, repo, msg)
}

func numbered(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("line ")
		b.WriteString(itoa(i))
		b.WriteByte('\n')
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}

func paths(files []FileStatus) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func equalStrings(a, b []string) bool {
	return equalLines(a, b)
}
