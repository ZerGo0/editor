package ghshared

import (
	"context"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/host"
)

type Session[C any] struct {
	Host     host.Host
	Ctx      context.Context
	Closed   bool
	Phase    Phase
	PhaseErr error
	ConnGen  int
	Wording  StateWording
	resolve  func(ctx context.Context, root, dataDir string) C
	apply    func(C)
	cancel   context.CancelFunc
}

func NewSession[C any](h host.Host, w StateWording, resolve func(ctx context.Context, root, dataDir string) C, apply func(C)) Session[C] {
	ctx, cancel := context.WithCancel(context.Background())
	return Session[C]{Host: h, Ctx: ctx, Wording: w, resolve: resolve, apply: apply, cancel: cancel}
}

func (s *Session[C]) Close() {
	s.Closed = true
	s.cancel()
}

func (s *Session[C]) Post(fn func()) {
	s.Host.Update(func() {
		if !s.Closed {
			fn()
		}
	})
}

func (s *Session[C]) AuthFailure(err error) {
	if IsUnauthorized(err) {
		s.Phase, s.PhaseErr = PhaseNotLoggedIn, err
	}
}

func (s *Session[C]) Fail(failure string, err error) {
	s.AuthFailure(err)
	s.Host.ShowError(failure, err)
}

func (s *Session[C]) Act(busy *string, status, failure string, work func(ctx context.Context) (func(), error)) {
	if *busy != "" {
		return
	}
	*busy = status
	applog.Go(func() {
		apply, err := work(s.Ctx)
		s.Post(func() {
			*busy = ""
			if err != nil {
				s.Fail(failure, err)
			}
			if apply != nil {
				apply()
			}
		})
	})
}

func (s *Session[C]) PostComment(busy *string, draft string, add func(ctx context.Context, body string) (github.Comment, error), posted func(github.Comment)) {
	body := strings.TrimSpace(draft)
	if body == "" {
		return
	}
	s.Act(busy, "Posting comment", "Cannot post comment", func(ctx context.Context) (func(), error) {
		comment, err := add(ctx, body)
		if err != nil {
			return nil, err
		}
		return func() { posted(comment) }, nil
	})
}

func (s *Session[C]) Connect() {
	s.ConnGen++
	gen := s.ConnGen
	s.Phase, s.PhaseErr = PhaseLoading, nil
	root, dataDir := s.Host.RootDir(), s.Host.UserDataDir()
	applog.Go(func() {
		conn := s.resolve(s.Ctx, root, dataDir)
		s.Post(func() {
			if gen == s.ConnGen {
				s.apply(conn)
			}
		})
	})
}

func Latest[C, R any](s *Session[C], gen *int, fetch func(ctx context.Context) R, apply func(R)) {
	*gen++
	current := *gen
	applog.Go(func() {
		res := fetch(s.Ctx)
		s.Post(func() {
			if current == *gen {
				apply(res)
			}
		})
	})
}
