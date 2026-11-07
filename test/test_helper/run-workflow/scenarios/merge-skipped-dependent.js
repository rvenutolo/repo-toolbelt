// The merge agent skips an issue (the user closed its PR mid-run): the record
// keeps the agent's note and lists the build worktree as left in place, and the
// dependent is skipped naming the issue, not parked and never built.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 2, batch: [L.issue(1), L.issue(2, { after: [1] })] })
exports.respond = L.responder({ 'merge #1': () => L.merge({ state: 'skipped', parkNote: 'The user closed the PR.' }) })
exports.check = (r, t, test) => {
  test('skipped by the merge agent', () => {
    L.assert.strictEqual(L.recOf(r, 1).state, 'skipped')
    L.assert.strictEqual(L.recOf(r, 1).note, 'The user closed the PR.')
    L.assert.deepStrictEqual(L.recOf(r, 1).leftInPlace, ['/main/.claude/worktrees/wf_1-1'])
  })
  test('the dependent is skipped naming it', () => {
    L.assert.strictEqual(L.recOf(r, 2).state, 'skipped')
    L.assert.strictEqual(L.recOf(r, 2).note, 'Not started: #1 did not merge: The user closed the PR.')
  })
  test('the dependent never built', () => L.assert.ok(!L.labels(t).includes('build #2')))
}
