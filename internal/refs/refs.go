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
func Resolve(repo string, in Input) ([]Target, error) {
	g := git{dir: repo}
	switch in.Hook {
	case hookPreCommit:
		return []Target{{Base: "HEAD"}}, nil
	case hookPrePush:
		return g.prePush(in)
	case hookCI:
		return g.ci(in)
	default:
		return nil, fmt.Errorf("%w: unknown hook %q", ErrUsage, in.Hook)
	}
}

func (g git) prePush(in Input) ([]Target, error) {
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
		head, err := g.commit(localSHA)
		if err != nil {
			return nil, fmt.Errorf("pre-push %s: %w", localRef, err)
		}
		base := ""
		// A remote sha this clone does not have (a force push from elsewhere)
		// leaves nothing to compare with, as a new branch does.
		if !isZero(remoteSHA) {
			if remote, err := g.commit(remoteSHA); err == nil {
				if base, err = g.mergeBase(remote, head); err != nil {
					return nil, fmt.Errorf("pre-push %s: %w", localRef, err)
				}
			}
		}
		if base == "" {
			base, err = g.mergeBaseWithRemoteHead(head, in.Remote)
			if err != nil {
				return nil, fmt.Errorf("pre-push %s: %w", localRef, err)
			}
		}
		targets = append(targets, Target{Ref: localRef, Base: base, Range: base + ".." + head})
	}
	return targets, nil
}

func (g git) ci(in Input) ([]Target, error) {
	if in.Base == "" && in.Head == "" {
		return []Target{{}}, nil
	}
	if in.Head == "" {
		return nil, fmt.Errorf("%w: --head is required with --base", ErrUsage)
	}
	head, err := g.commit(in.Head)
	if err != nil {
		return nil, fmt.Errorf("--head: %w", err)
	}
	base := ""
	if in.Base != "" && !isZero(in.Base) {
		if b, err := g.commit(in.Base); err == nil {
			if base, err = g.mergeBase(b, head); err != nil {
				return nil, err
			}
		}
	}
	if base == "" {
		if in.DefaultBranch == "" {
			return nil, fmt.Errorf("%w: --default-branch is required when --base is empty, all zeros or not in the checkout", ErrUsage)
		}
		def, err := g.commit("refs/remotes/origin/" + in.DefaultBranch)
		if err != nil {
			return nil, fmt.Errorf("origin/%s is not in the checkout; fetch it: %w", in.DefaultBranch, err)
		}
		if base, err = g.mergeBase(def, head); err != nil {
			return nil, err
		}
	}
	return []Target{{Base: base, Range: base + ".." + head}}, nil
}

func (g git) mergeBaseWithRemoteHead(head, remote string) (string, error) {
	remoteHead, err := g.commit("refs/remotes/" + remote + "/HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot find refs/remotes/%[1]s/HEAD to compare a new ref with; run `git remote set-head %[1]s --auto`: %[2]w", remote, err)
	}
	return g.mergeBase(remoteHead, head)
}

// isZero reports whether sha is git's all-zeros object name: no object.
func isZero(sha string) bool { return strings.Trim(sha, "0") == "" }

// git runs git in a repository, in the C locale.
type git struct{ dir string }

func (g git) output(args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed program; the arguments are this package's own or are object names it has verified
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
func (g git) commit(rev string) (string, error) {
	sha, err := g.output("rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("%q is not a commit in the checkout: %w", rev, err)
	}
	return sha, nil
}

// mergeBase takes two full object names, so neither can be read as an option.
func (g git) mergeBase(a, b string) (string, error) {
	sha, err := g.output("merge-base", a, b)
	if err != nil {
		return "", fmt.Errorf("%s and %s have no common ancestor; a shallow checkout needs full history (fetch-depth: 0): %w", a, b, err)
	}
	return sha, nil
}
