// Package refs turns what is running the lints (a git hook or CI) into the
// diff base and commit range each lint judges.
package refs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// errNotCommit means git ran fine and the name is not a commit in the checkout.
var errNotCommit = errors.New("not a commit in the checkout")

// ErrUsage marks an error that is the caller's mistake, such as a missing
// flag. Any other error is a setup problem in the checkout.
var ErrUsage = errors.New("usage")

const (
	hookPreCommit = "pre-commit"
	hookPrePush   = "pre-push"
	hookCI        = "ci"
)

var hexSHA = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Input is what Resolve works from.
type Input struct {
	// Hook is pre-commit, pre-push or ci.
	Hook string
	// Base and Head are, in ci, the raw event revisions: a pull request's
	// base and head SHAs, or a push's before and sha. Both empty means the
	// caller has no revisions to give.
	Base, Head string
	// DefaultBranch names the branch ci falls back to when Base is unusable.
	DefaultBranch string
	// Remote is the remote pre-push is pushing to.
	Remote string
	// PrePushLines are the lines git writes to a pre-push hook's stdin:
	// "<local ref> <local sha> <remote ref> <remote sha>".
	PrePushLines []string
}

// Target is one diff base and commit range to run the lints against.
type Target struct {
	// Ref names the pushed ref the target judges; empty unless pre-push.
	Ref string
	// Base is TOOLBELT_BASE and Range is TOOLBELT_RANGE.
	Base, Range string
}

// Resolve returns the targets the lints run against in the repository at
// repo. pre-push yields one target per pushed ref that is not a deletion, so
// possibly none.
func Resolve(ctx context.Context, repo string, in Input) ([]Target, error) {
	g := git{dir: repo}
	switch in.Hook {
	case hookPreCommit:
		return []Target{{Base: "HEAD"}}, nil
	case hookPrePush:
		return g.prePush(ctx, in)
	case hookCI:
		return g.ci(ctx, in)
	default:
		return nil, fmt.Errorf("%w: unknown hook %q", ErrUsage, in.Hook)
	}
}

func (g git) prePush(ctx context.Context, in Input) ([]Target, error) {
	if in.Remote == "" {
		return nil, fmt.Errorf("%w: --remote is required for pre-push", ErrUsage)
	}
	var targets []Target
	for _, line := range in.PrePushLines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 4 {
			return nil, fmt.Errorf("pre-push line %q: want <local ref> <local sha> <remote ref> <remote sha>", line)
		}
		localRef, localSHA, remoteSHA := fields[0], fields[1], fields[3]
		if !hexSHA.MatchString(localSHA) || !hexSHA.MatchString(remoteSHA) {
			return nil, fmt.Errorf("pre-push line %q: not a pair of object names", line)
		}
		if isZero(localSHA) {
			continue
		}
		head, err := g.commit(ctx, localSHA)
		if err != nil {
			return nil, fmt.Errorf("pre-push %s: %w", localRef, err)
		}
		base := ""
		// A remote sha this clone does not have (a force push from elsewhere)
		// leaves nothing to compare with, as a new branch does.
		if !isZero(remoteSHA) {
			remote, err := g.commit(ctx, remoteSHA)
			switch {
			case err == nil:
				base, err = g.mergeBase(ctx, remote, head)
				if err != nil {
					return nil, fmt.Errorf("pre-push %s: %w", localRef, err)
				}
				targets = append(targets, Target{Ref: localRef, Base: base, Range: remote + ".." + head})
				continue
			case !errors.Is(err, errNotCommit):
				return nil, fmt.Errorf("pre-push %s: %w", localRef, err)
			}
		}
		if base == "" {
			base, err = g.mergeBaseWithRemoteHead(ctx, head, in.Remote)
			if err != nil {
				return nil, fmt.Errorf("pre-push %s: %w", localRef, err)
			}
		}
		targets = append(targets, Target{Ref: localRef, Base: base, Range: base + ".." + head})
	}
	return targets, nil
}

func (g git) ci(ctx context.Context, in Input) ([]Target, error) {
	if in.Base == "" && in.Head == "" {
		return []Target{{}}, nil
	}
	if in.Head == "" {
		return nil, fmt.Errorf("%w: --head is required with --base", ErrUsage)
	}
	head, err := g.commit(ctx, in.Head)
	if err != nil {
		return nil, fmt.Errorf("--head: %w", err)
	}
	if in.Base != "" && !isZero(in.Base) {
		b, err := g.commit(ctx, in.Base)
		switch {
		case err == nil:
			return g.ciWithBase(ctx, b, head)
		case !errors.Is(err, errNotCommit):
			return nil, fmt.Errorf("--base: %w", err)
		}
	}
	if in.DefaultBranch == "" {
		return nil, fmt.Errorf("%w: --default-branch is required when --base is empty, all zeros or not in the checkout", ErrUsage)
	}
	def, err := g.commit(ctx, "refs/remotes/origin/"+in.DefaultBranch)
	if err != nil {
		return nil, fmt.Errorf("origin/%s is not in the checkout; fetch it: %w", in.DefaultBranch, err)
	}
	base, err := g.mergeBase(ctx, def, head)
	if err != nil {
		return nil, err
	}
	return []Target{{Base: base, Range: base + ".." + head}}, nil
}

// ciWithBase diffs the checkout against its merge base with base, and takes
// the range from base's merge base with head. On a pull request the checkout
// is GitHub's merge of head into base, so diffing it against the branch point
// would count base's own commits since then as changes; the range still ends
// at head, so the synthetic merge commit is not judged. On a push the
// checkout is head and both merge bases agree.
func (g git) ciWithBase(ctx context.Context, base, head string) ([]Target, error) {
	checkout, err := g.commit(ctx, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("the checkout's HEAD: %w", err)
	}
	diffBase, err := g.mergeBase(ctx, base, checkout)
	if err != nil {
		return nil, err
	}
	rangeBase, err := g.mergeBase(ctx, base, head)
	if err != nil {
		return nil, err
	}
	return []Target{{Base: diffBase, Range: rangeBase + ".." + head}}, nil
}

func (g git) mergeBaseWithRemoteHead(ctx context.Context, head, remote string) (string, error) {
	remoteHead, err := g.commit(ctx, "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot find refs/remotes/%[1]s/HEAD to compare a new ref with; run `git remote set-head %[1]s --auto`: %[2]w", remote, err)
	}
	return g.mergeBase(ctx, remoteHead, head)
}

// isZero reports whether sha is git's all-zeros object name: no object.
func isZero(sha string) bool { return strings.Trim(sha, "0") == "" }

// git runs git in a repository, in the C locale.
type git struct{ dir string }

func (g git) output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed program; the arguments are this package's own or are object names it has verified
	cmd.Dir = g.dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=")
	out, err := cmd.Output()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		msg, _, _ := strings.Cut(strings.TrimSpace(string(exitErr.Stderr)), "\n")
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// commit returns the full object name of the commit rev names, peeling tags.
func (g git) commit(ctx context.Context, rev string) (string, error) {
	sha, err := g.output(ctx, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		// --quiet --verify exits 1, silently, for a name that is not a commit;
		// every other failure is git itself failing.
		return "", fmt.Errorf("%q: %w", rev, errNotCommit)
	}
	if err != nil {
		return "", err
	}
	return sha, nil
}

// mergeBase takes two full object names, so neither can be read as an option.
func (g git) mergeBase(ctx context.Context, a, b string) (string, error) {
	sha, err := g.output(ctx, "merge-base", a, b)
	if err != nil {
		return "", fmt.Errorf("%s and %s have no common ancestor; a shallow checkout needs full history (fetch-depth: 0): %w", a, b, err)
	}
	return sha, nil
}
