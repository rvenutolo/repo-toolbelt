// The merge agent asks for a resolution review three times in a row: the
// review runs between calls with the inputs agents.md lists, pendingRulings
// carry forward without Summary: lines, the merge worktree enters the record
// once, and the third request parks the issue with the three-changes note and
// the divergence sentence leftovers.md keys on.
const L = require('../lib')

let k = 0
exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1)] })
exports.respond = L.responder({
  'merge #1': () => {
    k++
    return L.merge({
      state: 'needs-review', reason: 'conflict', fixBase: `sha${k}`, mergeWorktree: '/main/.claude/worktrees/merge-1',
      conflictedFiles: ['a.go'], rulings: [`Ruling: r${k}`, `Summary: s${k}`],
    })
  },
  'review #1 resolution': () => L.review({ changeFindings: [{ file: 'a.go', summary: 'lossy' }] }),
})
exports.check = (r, t, test) => {
  test('call sequence', () => L.assert.deepStrictEqual(L.labels(t), ['build #1', 'review #1 r1', 'merge #1',
    'review #1 resolution a1', 'merge #1 a2', 'review #1 resolution a2', 'merge #1 a3']))
  test('parked with the three-changes note and the divergence sentence', () => {
    const rec = L.recOf(r, 1)
    L.assert.strictEqual(rec.state, 'parked')
    L.assert.match(rec.note, /^Three changes needing review in a row/)
    L.assert.match(rec.note,
      /The worktree \/main\/\.claude\/worktrees\/merge-1 holds an unpushed, rebased fix\/1-x that diverges from origin\/fix\/1-x\.$/)
  })
  test('pendingRulings carry forward without Summary lines', () => {
    const second = L.inputsOf(t.calls[4].prompt)
    L.assert.deepStrictEqual(second.pendingRulings, ['Ruling: r1'])
    L.assert.strictEqual(second.resolutionReviewed, true)
    L.assert.deepStrictEqual(second.resolutionFindings, [{ file: 'a.go', summary: 'lossy' }])
    L.assert.deepStrictEqual(L.inputsOf(t.calls[6].prompt).pendingRulings, ['Ruling: r1', 'Ruling: r2'])
  })
  test('resolution review inputs', () => {
    L.assert.deepStrictEqual(Object.keys(L.inputsOf(t.calls[3].prompt)).sort(),
      ['branch', 'conflictedFiles', 'failure', 'fixBase', 'main', 'mergeWorktree', 'n', 'pr', 'reason', 'resolution'])
  })
  test('merge worktree recorded once', () => L.assert.deepStrictEqual(L.recOf(r, 1).worktrees,
    ['/main/.claude/worktrees/wf_1-1', '/main/.claude/worktrees/merge-1']))
  test('rulings accumulate', () => L.assert.strictEqual(L.recOf(r, 1).rulings.length, 6))
}
