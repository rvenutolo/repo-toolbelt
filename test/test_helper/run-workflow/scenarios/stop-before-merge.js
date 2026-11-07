// stopBeforeMerge (the shakedown): the merge agent still runs, with
// stopAtMerge true, so the shakedown exercises the merge queue's rebase, gate
// and CI; it returns ready-to-merge without merging, the issue ends
// ready-to-merge with its note and its merge worktree recorded, and a
// dependent is skipped with the "waits for the user's OK" note rather than
// built on unmerged code.
const L = require('../lib')

const wt = '/main/.claude/worktrees/merge-1'
exports.args = L.baseArgs({ cap: 1, stopBeforeMerge: true, batch: [L.issue(1), L.issue(2, { after: [1] })] })
exports.respond = L.responder({
  'merge #1': () => L.merge({ state: 'ready-to-merge', mergeWorktree: wt }),
})
exports.check = (r, t, test) => {
  test('ready to merge', () => L.assert.deepStrictEqual(r.readyToMerge, [1]))
  test('its note', () => L.assert.strictEqual(L.recOf(r, 1).note, "Reviewed; waits for the user's OK to merge."))
  test('the merge worktree is recorded', () => L.assert.ok(L.recOf(r, 1).worktrees.includes(wt)))
  test('the dependent is skipped, not started', () => {
    L.assert.deepStrictEqual(r.skipped, [2])
    L.assert.strictEqual(L.recOf(r, 2).note, "Not started: #1 waits for the user's OK to merge.")
  })
  test('the merge agent ran with stopAtMerge', () => {
    const call = t.calls.find((c) => c.label === 'merge #1')
    L.assert.ok(call, 'no merge agent ran')
    L.assert.strictEqual(L.inputsOf(call.prompt).stopAtMerge, true)
  })
  test('nothing is left in place', () => L.assert.deepStrictEqual(L.recOf(r, 1).leftInPlace, []))
}
