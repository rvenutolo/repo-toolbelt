// The merge agent's BEHIND/DIRTY retry count crosses merge calls: each call
// gets the total the previous call returned, so the third retry on one PR
// parks it however the retries fall across calls, as serial merging.md's
// third-retry rule does within one session.
const L = require('../lib')

let k = 0
exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1)] })
exports.respond = L.responder({
  'merge #1': () => {
    k++
    if (k < 3) {
      return L.merge({ state: 'needs-review', reason: 'conflict', fixBase: `sha${k}`, mergeRetries: k,
        mergeWorktree: '/main/.claude/worktrees/merge-1', conflictedFiles: ['a.go'] })
    }
    return L.merge({ mergeRetries: 2 })
  },
  'review #1 resolution': () => L.review(),
})
exports.check = (r, t, test) => {
  test('mergeRetries threaded from each call to the next', () => {
    const calls = t.calls.filter((call) => call.label.startsWith('merge #1'))
    L.assert.deepStrictEqual(calls.map((call) => L.inputsOf(call.prompt).mergeRetries), [0, 1, 2])
  })
  test('merged', () => L.assert.deepStrictEqual(r.merged, [1]))
}
