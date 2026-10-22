package refs_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/refs"
)

const zeros = "0000000000000000000000000000000000000000"

// fixture is a work repository whose remote "origin" is a bare repository
// beside it. main holds c1 and c2 and is pushed, with refs/remotes/origin/HEAD
// set.
type fixture struct {
	t       *testing.T
	repo    string
	remote  string
	c1, c2  string
	counter int
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{t: t, repo: filepath.Join(root, "work"), remote: filepath.Join(root, "origin.git")}
	git(t, root, "init", "--quiet", "--bare", "--initial-branch=main", f.remote)
	git(t, root, "init", "--quiet", "--initial-branch=main", f.repo)
	git(t, f.repo, "remote", "add", "origin", f.remote)
	f.c1 = f.commit()
	f.c2 = f.commit()
	git(t, f.repo, "push", "--quiet", "origin", "main")
	git(t, f.repo, "remote", "set-head", "origin", "main")
	return f
}

// commit adds a commit with a unique file to the current branch.
func (f *fixture) commit() string {
	f.t.Helper()
	f.counter++
	name := filepath.Join(f.repo, "f"+strings.Repeat("x", f.counter))
	if err := os.WriteFile(name, []byte("x\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	git(f.t, f.repo, "add", "--all")
	git(f.t, f.repo, "commit", "--quiet", "--message=m")
	return git(f.t, f.repo, "rev-parse", "HEAD")
}

// branch creates a branch at from, checks it out and adds n commits, returning
// the last.
func (f *fixture) branch(name, from string, n int) string {
	f.t.Helper()
	git(f.t, f.repo, "checkout", "--quiet", "-B", name, from)
	tip := from
	for range n {
		tip = f.commit()
	}
	return tip
}

func resolve(f *fixture, in refs.Input) ([]refs.Target, error) {
	return refs.Resolve(f.t.Context(), f.repo, in)
}

// corrupt overwrites the loose object sha so git fails reading it, as opposed
// to not finding it.
func (f *fixture) corrupt(sha string) {
	f.t.Helper()
	path := filepath.Join(f.repo, ".git", "objects", sha[:2], sha[2:])
	if err := os.Chmod(path, 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("garbage"), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func TestResolve_preCommitDiffsAgainstHead(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	got, err := resolve(f, refs.Input{Hook: "pre-commit"})
	want := []refs.Target{{Base: "HEAD"}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushExistingBranchUsesMergeBaseWithRemoteSha(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	git(t, f.repo, "push", "--quiet", "origin", "feature")
	f2 := f.commit()
	line := "refs/heads/feature " + f2 + " refs/heads/feature " + f1
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
	want := []refs.Target{{Ref: "refs/heads/feature", Base: f1, Range: f1 + ".." + f2}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushRewrittenBranchRangeStartsAtRemoteShaBaseIsMergeBase(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	git(t, f.repo, "push", "--quiet", "origin", "feature")
	g1 := f.branch("feature", f.c2, 1)
	line := "refs/heads/feature " + g1 + " refs/heads/feature " + f1
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
	want := []refs.Target{{Ref: "refs/heads/feature", Base: f.c2, Range: f1 + ".." + g1}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushNewBranchUsesRemoteHead(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 2)
	line := "refs/heads/feature " + f1 + " refs/heads/feature " + zeros
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
	want := []refs.Target{{Ref: "refs/heads/feature", Base: f.c2, Range: f.c2 + ".." + f1}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushRemoteShaUnknownLocallyFallsBackToRemoteHead(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	unknown := strings.Repeat("ab", 20)
	line := "refs/heads/feature " + f1 + " refs/heads/feature " + unknown
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
	want := []refs.Target{{Ref: "refs/heads/feature", Base: f.c2, Range: f.c2 + ".." + f1}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushNewBranchWithoutRemoteHeadNamesTheFix(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	git(t, f.repo, "remote", "set-head", "origin", "--delete")
	line := "refs/heads/feature " + f1 + " refs/heads/feature " + zeros
	_, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
	if err == nil || !strings.Contains(err.Error(), "git remote set-head origin --auto") {
		t.Errorf("err = %v, want it to name `git remote set-head origin --auto`", err)
	}
}

func TestResolve_prePushDeletionIsSkipped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	lines := []string{
		"(delete) " + zeros + " refs/heads/gone " + f.c1,
		"refs/heads/feature " + f1 + " refs/heads/feature " + zeros,
	}
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: lines})
	want := []refs.Target{{Ref: "refs/heads/feature", Base: f.c2, Range: f.c2 + ".." + f1}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushOnlyDeletionsYieldNoTargets(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	line := "(delete) " + zeros + " refs/heads/gone " + f.c1
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
	if err != nil || len(got) != 0 {
		t.Errorf("Resolve = %v, %v; want no targets", got, err)
	}
}

func TestResolve_prePushTagsAreJudgedLikeBranches(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	git(t, f.repo, "tag", "light", f1)
	git(t, f.repo, "tag", "--annotate", "--message=m", "annotated", f1)
	annotated := git(t, f.repo, "rev-parse", "annotated")
	lines := []string{
		"refs/tags/light " + f1 + " refs/tags/light " + zeros,
		"refs/tags/annotated " + annotated + " refs/tags/annotated " + zeros,
	}
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: lines})
	want := []refs.Target{
		{Ref: "refs/tags/light", Base: f.c2, Range: f.c2 + ".." + f1},
		{Ref: "refs/tags/annotated", Base: f.c2, Range: f.c2 + ".." + f1},
	}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushOneTargetPerRefInOrder(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := f.branch("a", f.c2, 1)
	b := f.branch("b", f.c1, 1)
	c := f.branch("c", f.c2, 2)
	lines := []string{
		"refs/heads/a " + a + " refs/heads/a " + zeros,
		"refs/heads/b " + b + " refs/heads/b " + zeros,
		"refs/heads/c " + c + " refs/heads/c " + zeros,
	}
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: lines})
	want := []refs.Target{
		{Ref: "refs/heads/a", Base: f.c2, Range: f.c2 + ".." + a},
		{Ref: "refs/heads/b", Base: f.c1, Range: f.c1 + ".." + b},
		{Ref: "refs/heads/c", Base: f.c2, Range: f.c2 + ".." + c},
	}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_prePushWithoutRemoteIsUsageError(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, err := resolve(f, refs.Input{Hook: "pre-push"})
	if !errors.Is(err, refs.ErrUsage) || !strings.Contains(err.Error(), "--remote") {
		t.Errorf("err = %v, want a usage error naming --remote", err)
	}
}

func TestResolve_prePushMalformedLine(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for name, line := range map[string]string{
		"three fields":           "refs/heads/a " + f.c1 + " refs/heads/a",
		"five fields":            "refs/heads/a " + f.c1 + " refs/heads/a " + f.c1 + " extra",
		"non-hex local":          "refs/heads/a nothex refs/heads/a " + zeros,
		"non-hex remote":         "refs/heads/a " + f.c1 + " refs/heads/a nothex",
		"local sha not a commit": "refs/heads/a " + strings.Repeat("cd", 20) + " refs/heads/a " + zeros,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
			if err == nil {
				t.Error("Resolve succeeded, want an error")
			}
		})
	}
}

func TestResolve_prePushIgnoresBlankLines(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	got, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{"", "  "}})
	if err != nil || len(got) != 0 {
		t.Errorf("Resolve = %v, %v; want no targets", got, err)
	}
}

func TestResolve_ciPullRequestHeadCheckoutBaseIsBranchPointNotMovedBase(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p1 := f.branch("pr", f.c2, 1)
	moved := f.branch("moved", f.c2, 1)
	git(t, f.repo, "checkout", "--quiet", "--detach", p1)
	got, err := resolve(f, refs.Input{Hook: "ci", Base: moved, Head: p1, DefaultBranch: "main"})
	want := []refs.Target{{Base: f.c2, Range: f.c2 + ".." + p1}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_ciPullRequestMergeCheckoutDiffsOnlyThePullRequest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p1 := f.branch("pr", f.c2, 1)
	prFile := git(t, f.repo, "diff", "--name-only", f.c2, p1)
	moved := f.branch("moved", f.c2, 1)
	// actions/checkout on pull_request checks out the merge of head into base.
	git(t, f.repo, "checkout", "--quiet", "--detach", moved)
	git(t, f.repo, "merge", "--quiet", "--no-ff", "--no-edit", p1)
	got, err := resolve(f, refs.Input{Hook: "ci", Base: moved, Head: p1, DefaultBranch: "main"})
	if err != nil || len(got) != 1 {
		t.Fatalf("Resolve = %v, %v; want one target", got, err)
	}
	if diff := git(t, f.repo, "diff", "--name-only", got[0].Base); diff != prFile {
		t.Errorf("git diff %s = %q, want only the pull request's change %q", got[0].Base, diff, prFile)
	}
	if want := f.c2 + ".." + p1; got[0].Range != want {
		t.Errorf("Range = %q, want %q", got[0].Range, want)
	}
}

func TestResolve_ciPushUsesMergeBaseOfBeforeAndSha(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	f2 := f.commit()
	got, err := resolve(f, refs.Input{Hook: "ci", Base: f1, Head: f2})
	want := []refs.Target{{Base: f1, Range: f1 + ".." + f2}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_ciUnusableBaseFallsBackToDefaultBranch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	for name, base := range map[string]string{
		"all zeros":      zeros,
		"not in repo":    strings.Repeat("ab", 20),
		"not a revision": "no such revision",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := resolve(f, refs.Input{Hook: "ci", Base: base, Head: f1, DefaultBranch: "main"})
			want := []refs.Target{{Base: f.c2, Range: f.c2 + ".." + f1}}
			if err != nil || !cmp.Equal(want, got) {
				t.Errorf("Resolve = %v, %v; want %v", got, err, want)
			}
		})
	}
}

func TestResolve_ciHeadWithoutBaseFallsBackToDefaultBranch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	got, err := resolve(f, refs.Input{Hook: "ci", Head: f1, DefaultBranch: "main"})
	want := []refs.Target{{Base: f.c2, Range: f.c2 + ".." + f1}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_ciFallbackNeedsDefaultBranch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	_, err := resolve(f, refs.Input{Hook: "ci", Base: zeros, Head: f1})
	if !errors.Is(err, refs.ErrUsage) || !strings.Contains(err.Error(), "--default-branch") {
		t.Errorf("err = %v, want a usage error naming --default-branch", err)
	}
}

func TestResolve_ciFallbackWithMissingDefaultBranchRef(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	_, err := resolve(f, refs.Input{Hook: "ci", Base: zeros, Head: f1, DefaultBranch: "nope"})
	if err == nil || errors.Is(err, refs.ErrUsage) || !strings.Contains(err.Error(), "origin/nope") {
		t.Errorf("err = %v, want a setup error naming origin/nope", err)
	}
}

func TestResolve_ciBaseWithoutHeadIsUsageError(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, err := resolve(f, refs.Input{Hook: "ci", Base: f.c1})
	if !errors.Is(err, refs.ErrUsage) || !strings.Contains(err.Error(), "--head") {
		t.Errorf("err = %v, want a usage error naming --head", err)
	}
}

func TestResolve_ciHeadMustBeACommit(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, err := resolve(f, refs.Input{Hook: "ci", Base: f.c1, Head: strings.Repeat("cd", 20)})
	if err == nil {
		t.Error("Resolve succeeded, want an error")
	}
}

func TestResolve_ciUnrelatedHistoriesNameShallowClone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	git(t, f.repo, "checkout", "--quiet", "--orphan", "other")
	other := f.commit()
	_, err := resolve(f, refs.Input{Hook: "ci", Base: f.c2, Head: other})
	if err == nil || !strings.Contains(err.Error(), "fetch-depth") {
		t.Errorf("err = %v, want it to mention fetch-depth", err)
	}
}

func TestResolve_ciWithoutRevisionsIsOneEmptyTarget(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	got, err := resolve(f, refs.Input{Hook: "ci"})
	want := []refs.Target{{}}
	if err != nil || !cmp.Equal(want, got) {
		t.Errorf("Resolve = %v, %v; want %v", got, err, want)
	}
}

func TestResolve_ciRevisionStartingWithDashIsNotAnOption(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, err := resolve(f, refs.Input{Hook: "ci", Base: f.c1, Head: "--output=" + filepath.Join(f.repo, "marker")})
	if err == nil {
		t.Error("Resolve succeeded, want an error")
	}
	if _, statErr := os.Stat(filepath.Join(f.repo, "marker")); statErr == nil {
		t.Error("an option in a revision was interpreted by git")
	}
}

func TestResolve_unknownHook(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, err := resolve(f, refs.Input{Hook: "post-merge"})
	if !errors.Is(err, refs.ErrUsage) {
		t.Errorf("err = %v, want a usage error", err)
	}
}

func TestResolve_prePushGitFailureReadingRemoteShaIsNotSwallowed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	g1 := f.branch("other", f.c2, 1)
	f.corrupt(f1)
	line := "refs/heads/other " + g1 + " refs/heads/feature " + f1
	_, err := resolve(f, refs.Input{Hook: "pre-push", Remote: "origin", PrePushLines: []string{line}})
	if err == nil || errors.Is(err, refs.ErrUsage) || !strings.Contains(err.Error(), "exit status 128") {
		t.Errorf("err = %v, want git's own failure carrying git's exit status", err)
	}
}

func TestResolve_ciGitFailureReadingBaseIsNotSwallowed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f1 := f.branch("feature", f.c2, 1)
	g1 := f.branch("other", f.c2, 1)
	f.corrupt(f1)
	_, err := resolve(f, refs.Input{Hook: "ci", Base: f1, Head: g1, DefaultBranch: "main"})
	if err == nil || errors.Is(err, refs.ErrUsage) || !strings.Contains(err.Error(), "exit status 128") {
		t.Errorf("err = %v, want git's own failure carrying git's exit status", err)
	}
}

func TestResolve_cancelledContextStopsGit(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := refs.Resolve(ctx, f.repo, refs.Input{Hook: "ci", Base: f.c1, Head: f.c2})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want it to wrap context.Canceled", err)
	}
}
