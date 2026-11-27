package applog

import (
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

type Panic struct {
	Where string
	Value any
	Stack []byte
}

func (p *Panic) Error() string {
	return fmt.Sprintf("%s: panic: %v", p.Where, p.Value)
}

var recovered struct {
	mu     sync.Mutex
	path   string
	notify func(*Panic)
}

func setCrashPath(path string) {
	recovered.mu.Lock()
	defer recovered.mu.Unlock()
	recovered.path = path
}

func OnRecovered(fn func(*Panic)) {
	recovered.mu.Lock()
	defer recovered.mu.Unlock()
	recovered.notify = fn
}

func Recover(where string, handle func(*Panic)) {
	v := recover()
	if v == nil {
		return
	}
	p := &Panic{Where: where, Value: v, Stack: debug.Stack()}
	notify := report(p)
	if handle == nil {
		handle = notify
	}
	if handle != nil {
		handle(p)
	}
}

func Go(fn func()) {
	go func() {
		defer Recover("goroutine", nil)
		fn()
	}()
}

func report(p *Panic) func(*Panic) {
	log.Printf("recovered %v", p)
	recovered.mu.Lock()
	defer recovered.mu.Unlock()
	if recovered.path == "" {
		log.Printf("%s", p.Stack)
		return recovered.notify
	}
	f, err := openCrash(recovered.path)
	if err != nil {
		log.Printf("%v", err)
		return recovered.notify
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s recovered %v\n\n%s\n", time.Now().Format(time.RFC3339), p, p.Stack); err != nil {
		log.Printf("write crash log: %v", err)
	}
	return recovered.notify
}
