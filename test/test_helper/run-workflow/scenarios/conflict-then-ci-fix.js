// A merge call that resolves a conflict, then one that fixes CI, each go to a
// resolution review; the next merge call gets resolutionReviewed, the pending
// rulings and the last reason, and the issue merges with the ruling kept.
const L = require('../lib')

let calls = 0
const seen = []
exports.args = L.baseArgs({ batch: [L.issue(1), L.issue(2)] })
exports.respond = L.responder({
  'merge #2': (label, prompt) => {
    calls += 1
    seen.push(L.inputsOf(prompt))
    if (calls === 1) {
      return L.merge({
        state: 'needs-review', reason: 'conflict', mergeWorktree: '/wt/2', fixBase: 'abc', conflictedFiles: ['a.toml'],
        rulings: ['Ruling: kept both entries'],
      })
    }
    if (calls === 2) {
      return L.merge({ state: 'needs-review', reason: 'ci', mergeWorktree: '/wt/2', fixBase: 'def', conflictedFiles: ['b.go'] })
    }
    return L.merge()
  },
})
exports.check = (r, t, test) => {
  test('both merge', () => L.assert.deepStrictEqual([...r.merged].sort(), [1, 2]))
  test('each change needing review gets a resolution review', () => {
    const labels = L.labels(t)
    L.assert.ok(labels.includes('review #2 resolution a1'))
    L.assert.ok(labels.includes('review #2 resolution a2'))
  })
  test('the second call carries the review and the pending ruling', () => {
    L.assert.strictEqual(seen[1].resolutionReviewed, true)
    L.assert.deepStrictEqual(seen[1].pendingRulings, ['Ruling: kept both entries'])
    L.assert.strictEqual(seen[1].lastReason, 'conflict')
  })
  test('the third call carries the ci reason', () => L.assert.strictEqual(seen[2].lastReason, 'ci'))
  test('the ruling is kept on the record', () => {
    L.assert.deepStrictEqual(L.recOf(r, 2).rulings, ['Ruling: kept both entries'])
  })
}
