//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	binaryName         = "editor-e2e"
	socketEnv          = "MYGO_AUTOMATION"
	userDataEnv        = "EDITOR_E2E_USER_DATA"
	artifactsEnv       = "EDITOR_E2E_ARTIFACTS"
	startTimeout       = 20 * time.Second
	quitTimeout        = 10 * time.Second
	waitTimeout        = 5 * time.Second
	pollInterval       = 50 * time.Millisecond
	launchPollInterval = 10 * time.Millisecond
	idleTimeoutMs      = 300
	socketTempRoot     = "/tmp"
	socketDirPrefix    = "editor-e2e-"
	codeTimeout        = "timeout"
	runtimeDirName     = "run"
	runtimeDirPerm     = 0o700
	runtimeDirEnv      = "XDG_RUNTIME_DIR"
)

var appBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", binaryName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	appBinary = filepath.Join(dir, binaryName)
	build := exec.Command("go", "build", "-tags", "e2e", "-o", appBinary, ".")
	build.Dir = ".."
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build app:", err)
		os.Exit(1)
	}
	code := m.Run()
	removeDemoCache()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Node struct {
	ID          string   `json:"id"`
	Parent      string   `json:"parent"`
	Role        string   `json:"role"`
	Key         any      `json:"key"`
	Label       string   `json:"label"`
	Value       string   `json:"value"`
	Description string   `json:"description"`
	States      []string `json:"states"`
	Actions     []string `json:"actions"`
	Bounds      Rect     `json:"bounds"`
}

func (n Node) Has(state string) bool {
	for _, s := range n.States {
		if s == state {
			return true
		}
	}
	return false
}

type Window struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Visible bool   `json:"visible"`
}

type App struct {
	t        *testing.T
	UserData string
	cmd      *exec.Cmd
	conn     net.Conn
	reader   *bufio.Reader
	output   *syncBuffer
	nextID   int
	done     chan struct{}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func Launch(t *testing.T, args ...string) *App {
	t.Helper()
	return LaunchWith(t, t.TempDir(), args...)
}

func LaunchWith(t *testing.T, userData string, args ...string) *App {
	t.Helper()
	return LaunchEnv(t, userData, nil, args...)
}

func LaunchEnv(t *testing.T, userData string, env []string, args ...string) *App {
	t.Helper()
	sockDir, err := os.MkdirTemp(socketTempRoot, socketDirPrefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	runtimeDir := filepath.Join(sockDir, runtimeDirName)
	if err := os.Mkdir(runtimeDir, runtimeDirPerm); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(sockDir, "app.sock")
	a := &App{t: t, UserData: userData, output: &syncBuffer{}, done: make(chan struct{})}
	a.cmd = exec.Command(appBinary, args...)
	a.cmd.Env = append(os.Environ(), socketEnv+"="+sock, userDataEnv+"="+userData, runtimeDirEnv+"="+runtimeDir)
	a.cmd.Env = append(a.cmd.Env, env...)
	a.cmd.Stdout, a.cmd.Stderr = a.output, a.output
	if err := a.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = a.cmd.Wait()
		close(a.done)
	}()
	t.Cleanup(a.Quit)
	deadline := time.Now().Add(startTimeout)
	for {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			a.conn, a.reader = conn, bufio.NewReader(conn)
			return a
		}
		select {
		case <-a.done:
			t.Fatalf("app exited before it listened: %s", a.output.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("app did not listen on %s: %v\n%s", sock, err, a.output.String())
		}
		time.Sleep(launchPollInterval)
	}
}

func (a *App) Quit() {
	select {
	case <-a.done:
		return
	default:
	}
	if a.conn != nil {
		_ = a.conn.Close()
	}
	_ = a.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-a.done:
	case <-time.After(quitTimeout):
		_ = a.cmd.Process.Kill()
		<-a.done
		a.t.Errorf("app did not quit within %s", quitTimeout)
	}
}

func (a *App) Output() string {
	return a.output.String()
}

type rpcError struct {
	Method  string
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return e.Method + ": " + e.Code + ": " + e.Message
}

func (a *App) Call(method string, params any) (json.RawMessage, error) {
	a.nextID++
	line, err := json.Marshal(map[string]any{"id": a.nextID, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if _, err := a.conn.Write(append(line, '\n')); err != nil {
		return nil, err
	}
	answer, err := a.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", method, err, a.output.String())
	}
	var res struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(answer, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		res.Error.Method = method
		return nil, res.Error
	}
	return res.Result, nil
}

func (a *App) must(method string, params any) json.RawMessage {
	a.t.Helper()
	res, err := a.Call(method, params)
	if err != nil {
		a.t.Fatalf("%v\napp output:\n%s", err, a.output.String())
	}
	return res
}

func (a *App) Windows() []Window {
	a.t.Helper()
	var res struct {
		Windows []Window `json:"windows"`
	}
	if err := json.Unmarshal(a.must("windows", nil), &res); err != nil {
		a.t.Fatal(err)
	}
	return res.Windows
}

func (a *App) WaitWindow(title string) int {
	a.t.Helper()
	var id int
	a.Eventually("window "+title, func() bool {
		for _, w := range a.Windows() {
			if w.Title == title {
				id = w.ID
				return true
			}
		}
		return false
	})
	return id
}

func (a *App) Idle(window int) {
	a.t.Helper()
	_, err := a.Call("idle", map[string]any{"window": window, "timeout": idleTimeoutMs})
	var rpc *rpcError
	if errors.As(err, &rpc) && rpc.Code == codeTimeout {
		return
	}
	if err != nil {
		a.t.Fatalf("%v\napp output:\n%s", err, a.output.String())
	}
}

func (a *App) Tree(window int) []Node {
	a.t.Helper()
	var res struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(a.must("tree", map[string]any{"window": window}), &res); err != nil {
		a.t.Fatal(err)
	}
	return res.Nodes
}

func (a *App) Find(window int, match func(Node) bool) (Node, bool) {
	a.t.Helper()
	for _, n := range a.Tree(window) {
		if match(n) {
			return n, true
		}
	}
	return Node{}, false
}

func (a *App) WaitNode(window int, what string, match func(Node) bool) Node {
	a.t.Helper()
	var found Node
	a.Eventually(what, func() bool {
		var ok bool
		found, ok = a.Find(window, match)
		return ok
	})
	return found
}

func (a *App) Eventually(what string, cond func() bool) {
	a.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for !cond() {
		if time.Now().After(deadline) {
			a.Screenshot(strings.ReplaceAll(what, " ", "-"))
			a.t.Fatalf("timed out waiting for %s\napp output:\n%s", what, a.output.String())
		}
		time.Sleep(pollInterval)
	}
}

func (a *App) Act(window int, id, action, text string) {
	a.t.Helper()
	params := map[string]any{"window": window, "id": id, "action": action}
	if text != "" {
		params["text"] = text
	}
	a.must("act", params)
}

func (a *App) Click(window int, n Node, clicks int) {
	a.t.Helper()
	x, y := n.Bounds.X+n.Bounds.Width/2, n.Bounds.Y+n.Bounds.Height/2
	a.must("input", map[string]any{"window": window, "events": []map[string]any{{"type": "click", "x": x, "y": y, "clicks": clicks}}})
	a.Idle(window)
}

func (a *App) Type(window int, text string) {
	a.t.Helper()
	a.must("input", map[string]any{"window": window, "events": []map[string]any{{"type": "text", "text": text}}})
	a.Idle(window)
}

func (a *App) Key(window int, key string) {
	a.t.Helper()
	a.must("input", map[string]any{"window": window, "events": []map[string]any{{"type": "key", "key": key}}})
	a.Idle(window)
}

func (a *App) Menu(window int, path ...string) {
	a.t.Helper()
	a.must("menu", map[string]any{"window": window, "path": path})
	a.Idle(window)
}

func (a *App) Capture() ([]byte, error) {
	res, err := a.Call("capture", nil)
	if err != nil {
		return nil, err
	}
	var shot struct {
		PNG string `json:"png"`
	}
	if err := json.Unmarshal(res, &shot); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(shot.PNG)
}

func (a *App) Screenshot(name string) string {
	a.t.Helper()
	dir := os.Getenv(artifactsEnv)
	if dir == "" {
		return ""
	}
	data, err := a.Capture()
	if err != nil {
		a.t.Logf("capture: %v", err)
		return ""
	}
	path := filepath.Join(dir, a.t.Name()+"-"+name+".png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.t.Logf("capture: %v", err)
		return ""
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		a.t.Logf("capture: %v", err)
		return ""
	}
	return path
}

func Project(t *testing.T, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func Label(label string) func(Node) bool {
	return func(n Node) bool { return n.Label == label }
}

func RoleLabel(role, label string) func(Node) bool {
	return func(n Node) bool { return n.Role == role && n.Label == label }
}
