package review

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CommandReview   = "review"
	CommandAnnotate = "annotate"
	CommandPlanHook = "plan-hook"
	RequestFlag     = "--review-request"
	ExitFailure     = 1
	ExitUsage       = 2
	pollInterval    = 100 * time.Millisecond
	gitDir          = ".git"
	maxHookInput    = 16 << 20
)

var Commands = []string{CommandReview, CommandAnnotate, CommandPlanHook}

var errUsage = errors.New("usage")

type Env struct {
	Ctx      context.Context
	UserData string
	PID      int
	Getwd    func() (string, error)
	Launch   func(requestPath string) error
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
}

type job struct {
	req    Request
	output func(Result) (string, error)
}

func IsCommand(name string) bool {
	for _, c := range Commands {
		if c == name {
			return true
		}
	}
	return false
}

func Run(name string, args []string, env Env) int {
	var j job
	var err error
	switch name {
	case CommandReview:
		j, err = reviewJob(args, env)
	case CommandAnnotate:
		j, err = annotateJob(args, env)
	case CommandPlanHook:
		j, err = planJob(args, env)
	default:
		err = errUsage
	}
	if errors.Is(err, errUsage) {
		return ExitUsage
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "editor %s: %v\n", name, err)
		return ExitFailure
	}
	j.req.PID = env.PID
	res, err := runSession(j.req, env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "editor %s: %v\n", name, err)
		return ExitFailure
	}
	out, err := j.output(res)
	if err != nil {
		fmt.Fprintf(env.Stderr, "editor %s: %v\n", name, err)
		return ExitFailure
	}
	fmt.Fprint(env.Stdout, out)
	return 0
}

func runSession(req Request, env Env) (Result, error) {
	s, err := NewSession(env.UserData)
	if err != nil {
		return Result{}, err
	}
	defer s.Remove()
	if err := s.Write(req); err != nil {
		return Result{}, err
	}
	if err := env.Launch(s.RequestPath()); err != nil {
		return Result{}, fmt.Errorf("open the editor: %w", err)
	}
	return s.Wait(env.Ctx, pollInterval)
}

func newFlags(name, usage, about string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: editor %s %s\n\n%s\n\n", name, usage, about)
		fs.PrintDefaults()
	}
	return fs
}

func parse(fs *flag.FlagSet, args []string, maxArgs int) error {
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if fs.NArg() > maxArgs {
		fs.Usage()
		return errUsage
	}
	return nil
}

func reviewJob(args []string, env Env) (job, error) {
	fs := newFlags(CommandReview, "[-json] [-diff <mode>] [-base <ref>] [folder | pull request URL]",
		"Opens the changes for review in the editor and prints the decision: approval, or the feedback to address.", env.Stderr)
	asJSON := fs.Bool("json", false, "print the decision as JSON: {\"decision\", \"message\"}")
	diff := fs.String("diff", string(DiffUncommitted), "the changes to review: "+joinModes())
	base := fs.String("base", "", "the branch to compare with in the branch mode (default: the default branch)")
	if err := parse(fs, args, 1); err != nil {
		return job{}, err
	}
	mode := DiffMode(*diff)
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	output := func(res Result) (string, error) { return ReviewOutput(res, *asJSON) }
	if pr, ok := ParsePRURL(fs.Arg(0)); ok {
		if set["diff"] || set["base"] {
			fmt.Fprintln(env.Stderr, "-diff and -base do not apply to a pull request")
			return job{}, errUsage
		}
		dir, err := env.Getwd()
		if err != nil {
			return job{}, err
		}
		folder, err := folderOf(dir)
		if err != nil {
			return job{}, err
		}
		return job{req: Request{Kind: KindReview, Folder: folder, PR: &pr, Notes: true}, output: output}, nil
	}
	if !mode.Valid() || set["base"] && mode != DiffBranch {
		fmt.Fprintf(env.Stderr, "-diff must be one of %s; -base needs -diff %s\n", joinModes(), DiffBranch)
		return job{}, errUsage
	}
	dir, err := argDir(fs.Arg(0), env)
	if err != nil {
		return job{}, err
	}
	root, ok := gitRoot(dir)
	if !ok {
		return job{}, fmt.Errorf("%s is not in a Git repository", dir)
	}
	return job{req: Request{Kind: KindReview, Folder: root, Diff: mode, Base: *base, Notes: true}, output: output}, nil
}

func annotateJob(args []string, env Env) (job, error) {
	fs := newFlags(CommandAnnotate, "[-gate] [-json] [-hook] <file.md>",
		"Opens a Markdown file for annotation on its source lines and prints the feedback.", env.Stderr)
	gate := fs.Bool("gate", false, "offer Approve, so the agent waits for an approval")
	asJSON := fs.Bool("json", false, "print the decision as JSON: {\"decision\", \"feedback\", \"annotationCount\", \"target\"}")
	hook := fs.Bool("hook", false, "print a hook decision: {\"decision\": \"block\", \"reason\"} for feedback, nothing else")
	if err := parse(fs, args, 1); err != nil {
		return job{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return job{}, errUsage
	}
	path, err := absFrom(fs.Arg(0), env)
	if err != nil {
		return job{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return job{}, err
	}
	if !info.Mode().IsRegular() {
		return job{}, fmt.Errorf("%s is not a file", path)
	}
	folder, err := folderOf(filepath.Dir(path))
	if err != nil {
		return job{}, err
	}
	flags := AnnotateFlags{JSON: *asJSON, Hook: *hook}
	req := Request{Kind: KindAnnotate, Folder: folder, File: path, Gate: *gate, Notes: *gate && !*hook}
	return job{req: req, output: func(res Result) (string, error) { return AnnotateOutput(path, res, flags) }}, nil
}

type hookInput struct {
	Cwd       string          `json:"cwd"`
	ToolInput json.RawMessage `json:"tool_input"`
}

type planInput struct {
	Plan         string `json:"plan"`
	PlanFilePath string `json:"planFilePath"`
}

func planJob(args []string, env Env) (job, error) {
	fs := newFlags(CommandPlanHook, "", "A Claude Code PermissionRequest hook for ExitPlanMode: reads the hook input on stdin, opens the plan for review and prints the decision.", env.Stderr)
	if err := parse(fs, args, 0); err != nil {
		return job{}, err
	}
	data, err := io.ReadAll(io.LimitReader(env.Stdin, maxHookInput))
	if err != nil {
		return job{}, err
	}
	var in hookInput
	if err := json.Unmarshal(data, &in); err != nil {
		return job{}, fmt.Errorf("read the hook input: %w", err)
	}
	var plan planInput
	if len(in.ToolInput) > 0 {
		if err := json.Unmarshal(in.ToolInput, &plan); err != nil {
			return job{}, fmt.Errorf("read tool_input: %w", err)
		}
	}
	if strings.TrimSpace(plan.Plan) == "" {
		return job{}, errors.New("the hook input has no tool_input.plan")
	}
	dir := in.Cwd
	if dir == "" {
		if dir, err = env.Getwd(); err != nil {
			return job{}, err
		}
	}
	folder, err := folderOf(dir)
	if err != nil {
		return job{}, err
	}
	req := Request{Kind: KindPlan, Folder: folder, Text: plan.Plan, File: plan.PlanFilePath}
	return job{req: req, output: func(res Result) (string, error) { return PlanHookOutput(req, in.ToolInput, res) }}, nil
}

func joinModes() string {
	names := make([]string, len(DiffModes))
	for i, m := range DiffModes {
		names[i] = string(m)
	}
	return strings.Join(names, ", ")
}

func absFrom(p string, env Env) (string, error) {
	if !filepath.IsAbs(p) {
		wd, err := env.Getwd()
		if err != nil {
			return "", err
		}
		p = filepath.Join(wd, p)
	}
	return filepath.EvalSymlinks(p)
}

func argDir(arg string, env Env) (string, error) {
	if arg == "" {
		arg = "."
	}
	dir, err := absFrom(arg, env)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", dir)
	}
	return dir, nil
}

func folderOf(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	if root, ok := gitRoot(dir); ok {
		return root, nil
	}
	return dir, nil
}

func gitRoot(dir string) (string, bool) {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, gitDir)); err == nil {
			return d, true
		}
		if filepath.Dir(d) == d {
			return "", false
		}
	}
}
