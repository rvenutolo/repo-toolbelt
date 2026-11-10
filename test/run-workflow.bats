#!/usr/bin/env bats

# The parallel backlog skill's workflow script, run under the stub harness in
# test/test_helper/run-workflow/: each scenario file stubs the Workflow tool's
# agent() with scripted results and asserts on the scheduler's result, so the
# slot cap, the serial merge queue, the budgets, batch halts, dependency skips
# and the retry-once rule are tested without an agent or a GitHub call.

function setup() {
  load test_helper/common
  HARNESS="${REPO_DIR}/test/test_helper/run-workflow/harness.js"
  SCENARIOS="${REPO_DIR}/test/test_helper/run-workflow/scenarios"
}

# @description Run one scenario and require the harness to report PASS.
# @arg $1 scenario the scenario file's basename
# @arg $2 case optional check-args case name
function run_scenario() {
  run node "${HARNESS}" "${SCENARIOS}/$1" "${@:2}"
  assert_success
  assert_output --partial 'PASS'
}

@test 'happy path: every issue merges, dependents build after their prerequisite, cap and serial merge hold' {
  run_scenario happy-path.js
}

@test 'a dead build agent is retried once, then parks with its dependent' {
  run_scenario dead-build-retried.js
}

@test 'a spent parked budget skips the build queued behind the cap' {
  run_scenario parked-budget.js
}

@test 'a batch halt from a review starts nothing further' {
  run_scenario review-batch-halt.js
}

@test 'three changes needing review in a row park the issue with the divergence sentence' {
  run_scenario needs-review-three-times.js
}

@test 'stopBeforeMerge runs the merge agent with stopAtMerge, ends ready-to-merge and skips its dependent' {
  run_scenario stop-before-merge.js
}

@test 'a build that reconciles to merged still runs the merge agent before its dependent builds' {
  run_scenario build-merged-runs-merge-tail.js
}

@test 'review, fix and merge prompts carry the piece, the halt answers and what a merge-time fix needs' {
  run_scenario prompts-carry-piece-and-halt-answers.js
}

@test 'a skipped prerequisite skips its dependents with the fixed notes' {
  run_scenario skipped-prerequisites.js
}

@test 'an issue the merge agent skips skips its dependent' {
  run_scenario merge-skipped-dependent.js
}

@test 'a fix agent takes a freed slot ahead of a waiting build' {
  run_scenario fix-priority.js
}

@test 'the second review round keeps the findings the first left unrecorded' {
  run_scenario review-round-two-unrecorded.js
}

@test 'a fix agent that finds its PR merged still runs the merge agent before its dependent builds' {
  run_scenario fix-merged-runs-merge-tail.js
}

@test 'the merge retry count threads from each merge call to the next' {
  run_scenario merge-retries-thread.js
}

@test 'lastReason threads between merge calls and a ci park says ahead of, not diverges' {
  run_scenario last-reason-ci-park.js
}

@test 'every build parking spends the parked budget, skips the queued builds and halts nothing' {
  run_scenario every-build-parks.js
}

@test 'a conflict resolution then a CI fix each get reviewed, and the rulings and reasons thread between merge calls' {
  run_scenario conflict-then-ci-fix.js
}

@test 'a dead merge agent is retried once, then parks with its dependent while another issue merges' {
  run_scenario dead-merge-retried.js
}

@test 'a merge agent that throws twice after a resolution parks with the diverges sentence' {
  run_scenario merge-throws-after-resolution.js
}

@test 'a build that returns ready without a PR parks with its dependent and starts no review' {
  run_scenario ready-without-pr-parks.js
}

@test 'a dead resolution review parks with the ahead-of sentence and the queue moves on' {
  run_scenario dead-resolution-review.js
}

@test 'two review rounds that both find changes go to the merge with no third review' {
  run_scenario review-two-rounds-cap.js
}

@test 'a two-issue shakedown ends both ready-to-merge, their merge agents one at a time with stopAtMerge' {
  run_scenario shakedown-two-independent.js
}

@test 'the stop file skips a build and its dependent, and leaves a merge ready-to-merge' {
  run_scenario stop-file.js
}

@test 'a resumed run returns finished calls from the cache and runs only the unfinished one live' {
  run_scenario resume-replays-finished-calls.js
}

@test 'checkArgs rejects a dependency cycle' {
  run_scenario check-args.js cycle
}

@test 'checkArgs rejects a self-cycle' {
  run_scenario check-args.js self-cycle
}

@test 'checkArgs rejects a prerequisite outside the batch' {
  run_scenario check-args.js after-outside-batch
}

@test 'checkArgs rejects a duplicate issue' {
  run_scenario check-args.js duplicate
}

@test 'checkArgs rejects a feature with no spec' {
  run_scenario check-args.js feature-without-spec
}

@test 'checkArgs rejects a deadline that is not a UTC time' {
  run_scenario check-args.js bad-deadline
}

@test 'checkArgs rejects a string issue number' {
  run_scenario check-args.js string-issue-number
}

@test 'checkArgs rejects a string in after' {
  run_scenario check-args.js string-after
}

@test 'checkArgs rejects a cap of zero' {
  run_scenario check-args.js cap-zero
}

@test 'checkArgs rejects a relative main checkout path' {
  run_scenario check-args.js relative-main
}

@test 'a failing scenario check fails the harness' {
  local scenario="${BATS_TEST_TMPDIR}/failing.js"
  cat > "${scenario}" << EOF
const L = require('${SCENARIOS}/../lib')
exports.args = L.baseArgs({ batch: [L.issue(1)] })
exports.respond = L.responder()
exports.check = (r, t, test) => test('wrong', () => L.assert.strictEqual(r.merged.length, 0))
EOF
  run node "${HARNESS}" "${scenario}"
  assert_failure 1
  assert_output --partial 'FAIL failing.js'
  assert_output --partial 'wrong:'
}
