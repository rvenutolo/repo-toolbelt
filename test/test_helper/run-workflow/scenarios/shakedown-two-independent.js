// The parallel shakedown's two independent issues: both build, both merge
// agents run with stopAtMerge one after the other, and both end
// ready-to-merge for the controller's own merges at Ending.
const L = require('../lib')

exports.args = L.baseArgs({ stopBeforeMerge: true, batch: [L.issue(1), L.issue(2)] })
exports.respond = L.responder({
  merge: (label) => L.merge({ state: 'ready-to-merge', mergeWorktree: `/wt/${label.match(/#(\d+)/)[1]}` }),
})
exports.check = (r, t, test) => {
  test('both are ready to merge', () => L.assert.deepStrictEqual([...r.readyToMerge].sort(), [1, 2]))
  test('nothing merges', () => L.assert.deepStrictEqual(r.merged, []))
  test('each merge agent ran with stopAtMerge', () => {
    const merges = t.calls.filter((c) => c.kind === 'merge')
    L.assert.strictEqual(merges.length, 2)
    for (const call of merges) L.assert.strictEqual(L.inputsOf(call.prompt).stopAtMerge, true)
  })
  test('the merge agents ran one at a time', () => L.assert.strictEqual(t.maxRunning.merge, 1))
  test('each keeps its merge worktree', () => {
    L.assert.ok(L.recOf(r, 1).worktrees.includes('/wt/1'))
    L.assert.ok(L.recOf(r, 2).worktrees.includes('/wt/2'))
  })
}
