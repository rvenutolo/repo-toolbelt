// stopBeforeMerge (the shakedown): a reviewed issue stops as ready-to-merge
// with its note, no merge agent runs, and a dependent is skipped with the
// "waits for the user's OK" note rather than built on unmerged code.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 1, stopBeforeMerge: true, batch: [L.issue(1), L.issue(2, { after: [1] })] })
exports.respond = L.responder()
exports.check = (r, t, test) => {
  test('ready to merge', () => L.assert.deepStrictEqual(r.readyToMerge, [1]))
  test('its note', () => L.assert.strictEqual(L.recOf(r, 1).note, "Reviewed; waits for the user's OK to merge."))
  test('the dependent is skipped, not started', () => {
    L.assert.deepStrictEqual(r.skipped, [2])
    L.assert.strictEqual(L.recOf(r, 2).note, "Not started: #1 waits for the user's OK to merge.")
  })
  test('no merge agent ran', () => L.assert.ok(!L.labels(t).some((label) => label.startsWith('merge'))))
}
