package git

import "context"

type MergeOptions struct {
	NoFF     bool
	Squash   bool
	NoCommit bool
}

func (r *Repo) Merge(ctx context.Context, ref string, o MergeOptions) (OpResult, error) {
	if err := checkRef(ref); err != nil {
		return OpResult{}, err
	}
	args := []string{"merge", "--no-edit"}
	if o.NoFF {
		args = append(args, "--no-ff")
	}
	if o.Squash {
		args = append(args, "--squash")
	}
	if o.NoCommit {
		args = append(args, "--no-commit")
	}
	out, err := r.write(ctx, append(args, ref)...)
	return r.finishOp(ctx, out, err)
}
