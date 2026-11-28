package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/ZerGo0/editor/internal/app"
	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/profiling"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/ZerGo0/editor/internal/telemetry"
	"github.com/egoist/mygo"
)

func main() {
	applyProfile()
	options := e2eOptions()
	if len(os.Args) > 1 && review.IsCommand(os.Args[1]) {
		os.Exit(runReview(os.Args[1], os.Args[2:]))
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	setupLogs()
	if err := app.New(options).Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func runReview(name string, args []string) int {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "editor %s: %v\n", name, err)
		return review.ExitFailure
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "editor %s: %v\n", name, err)
		return review.ExitFailure
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return review.Run(name, args, review.Env{
		Ctx: ctx, UserData: dir, PID: os.Getpid(), Getwd: os.Getwd, Launch: func(path string) error { return launchApp(exe, path) },
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
	})
}

func launchApp(exe, requestPath string) error {
	cmd := exec.Command(exe, review.RequestFlag, requestPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func setupLogs() {
	dir, err := mygo.App.Path(mygo.PathLogs)
	if err == nil {
		err = applog.Setup(dir)
		profiling.Listen(dir)
		if terr := telemetry.Setup(dir, mygo.App.Version()); terr != nil {
			log.Printf("telemetry: %v", terr)
		}
	}
	if err != nil {
		log.Printf("logs: %v", err)
	}
	log.Printf("start %s %s on %s/%s", mygo.App.Name(), mygo.App.Version(), runtime.GOOS, runtime.GOARCH)
}
