package session

import "context"

type Committer interface {
	Commit(context.Context, string) error
}

type CommitFunc func(context.Context, string) error

func (f CommitFunc) Commit(ctx context.Context, transcript string) error {
	return f(ctx, transcript)
}
