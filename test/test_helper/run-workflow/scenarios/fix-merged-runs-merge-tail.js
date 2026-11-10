// A fix agent that finds its PR already merged (a resumed run whose merge
// agent died after `gh pr merge`) goes through the merge queue like a build
// that reconciles to merged, so local main is pulled and the worktrees removed
// before the dependent builds.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 2, batch: [L.issue(1), L.issue(2, { after: [1] })] })
exports.respond = L.responder({
  'review #1 r1': () => L.review({ changeFindings: [{ file: 'f', summary: 's' }] }),
  'fix #1 r1': () => L.buildReady(1, { state: 'merged', worktree: '/main/.claude/worktrees/wf_1-fix1', branch: '', pr: '' }),
})
exports.check = (r, t, test) => {
  test('both merged', () => L.assert.deepStrictEqual([...r.merged].sort(), [1, 2]))
  test('a merge agent ran for the issue its fix found merged', () => L.assert.ok(L.labels(t).includes('merge #1')))
  test('the dependent builds after that merge agent ended', () => {
    const o = t.order
    L.assert.ok(o.indexOf('end merge #1') < o.indexOf('start build #2'), o.join(','))
  })
  test('both worktrees are recorded for cleanup', () =>
    L.assert.deepStrictEqual(L.recOf(r, 1).worktrees,
      ['/main/.claude/worktrees/wf_1-1', '/main/.claude/worktrees/wf_1-fix1']))
}
