// A merge call that throws twice after a reviewed conflict resolution parks
// the issue with the "diverges" sentence naming the merge worktree, and the
// next issue in the queue still merges.
const L = require('../lib')

let calls = 0
exports.args = L.baseArgs({ batch: [L.issue(1), L.issue(2)] })
exports.respond = L.responder({
  'merge #1': () => {
    calls += 1
    if (calls === 1) return L.merge({ state: 'needs-review', reason: 'conflict', mergeWorktree: '/wt/1', fixBase: 'a' })
    throw new Error('stalled')
  },
})
exports.check = (r, t, test) => {
  test('it parks with the failure and the diverges sentence', () => {
    L.assert.strictEqual(L.recOf(r, 1).note, 'The merge agent failed twice: Error: stalled. The worktree /wt/1 ' +
      'holds an unpushed, rebased fix/1-x that diverges from origin/fix/1-x.')
  })
  test('the merge worktree is recorded', () => L.assert.ok(L.recOf(r, 1).worktrees.includes('/wt/1')))
  test('the next issue merges', () => L.assert.deepStrictEqual(r.merged, [2]))
}
