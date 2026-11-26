package git

import (
	"context"
	"errors"
	"fmt"
)

type ResetMode string

const (
	ResetSoft  ResetMode = "soft"
	ResetMixed ResetMode = "mixed"
	ResetHard  ResetMode = "hard"
	ResetKeep  ResetMode = "keep"
)

func (r *Repo) CherryPick(ctx context.Context, oids ...string) (OpResult, error) {
	return r.sequence(ctx, "cherry-pick", oids)
}

func (r *Repo) Revert(ctx context.Context, oids ...string) (OpResult, error) {
	return r.sequence(ctx, "revert", oids)
}

func (r *Repo) sequence(ctx context.Context, verb string, oids []string) (OpResult, error) {
	if len(oids) == 0 {
		return OpResult{}, errors.New("git " + verb + ": no commits given")
	}
	if err := checkRefs(oids...); err != nil {
		return OpResult{}, err
	}
	args := []string{verb}
	if verb == "revert" {
		args = append(args, "--no-edit")
	}
	out, err := r.write(ctx, append(args, oids...)...)
	return r.finishOp(ctx, out, err)
}

func (r *Repo) Reset(ctx context.Context, rev string, mode ResetMode) error {
	if err := checkRef(rev); err != nil {
		return err
	}
	switch mode {
	case ResetSoft, ResetMixed, ResetHard, ResetKeep:
	default:
		return fmt.Errorf("git reset: unknown mode %q", mode)
	}
	_, err := r.write(ctx, "reset", "--"+string(mode), rev)
	return err
}
