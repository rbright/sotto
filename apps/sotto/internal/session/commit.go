package session

import "context"

// Committer persists/dispatches a transcript when session stop succeeds.
type Committer interface {
	Commit(context.Context, string) error
}

// CommitFunc adapts a function to the Committer interface.
type CommitFunc func(context.Context, string) error

func (f CommitFunc) Commit(ctx context.Context, transcript string) error {
	return f(ctx, transcript)
}
