// A build agent that reconciles to merged (a resumed run whose merge agent
// died after `gh pr merge`) still goes through the merge queue, whose section 1
// sends a MERGED PR to the pull and cleanup, so local main is pulled and the
// worktrees removed before the dependent builds; no review runs for it.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 2, batch: [L.issue(1), L.issue(2, { after: [1] })] })
exports.respond = L.responder({
  'build #1': () => L.buildReady(1, { state: 'merged' }),
})
exports.check = (r, t, test) => {
  test('both merged', () => L.assert.deepStrictEqual([...r.merged].sort(), [1, 2]))
  test('a merge agent ran for the reconciled issue', () => L.assert.ok(L.labels(t).includes('merge #1')))
  test('no review for it', () => L.assert.ok(!L.labels(t).some((label) => label.startsWith('review #1'))))
  test('the dependent builds after that merge agent ended', () => {
    const o = t.order
    L.assert.ok(o.indexOf('end merge #1') < o.indexOf('start build #2'), o.join(','))
  })
  test('its worktree is recorded for cleanup', () =>
    L.assert.deepStrictEqual(L.recOf(r, 1).worktrees, ['/main/.claude/worktrees/wf_1-1']))
}
