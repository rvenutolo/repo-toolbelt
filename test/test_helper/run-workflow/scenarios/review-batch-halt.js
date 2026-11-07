// At cap 1 a review reports a batch-level failure while two builds wait behind
// the cap: nothing starts after it, every issue's note leads with the halt, the
// review's out-of-scope findings stay in the record as unrecorded, and no merge
// agent runs.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1), L.issue(2), L.issue(3)] })
exports.respond = L.responder({
  'review #1 r1': () => L.review({ batchHalt: 'signing failed', otherFindings: [L.finding('x')] }),
})
exports.check = (r, t, test) => {
  test('halt recorded', () => L.assert.strictEqual(r.batchHalt, 'signing failed'))
  test('nothing starts after the halt', () => {
    const o = t.order
    const later = o.slice(o.indexOf('end review #1 r1')).filter((step) => step.startsWith('start'))
    L.assert.deepStrictEqual(later, [], later.join(','))
  })
  test('the reviewed issue parks with the halt note', () => {
    L.assert.strictEqual(L.recOf(r, 1).note, 'Batch halted: signing failed')
  })
  test('its other findings stay unrecorded', () => L.assert.strictEqual(L.recOf(r, 1).unrecorded.length, 1))
  test('the queued issues park with the halt note', () => {
    for (const n of [2, 3]) L.assert.strictEqual(L.recOf(r, n).note, 'Batch halted: signing failed')
  })
  test('no merge agent ran', () => L.assert.ok(!L.labels(t).some((label) => label.startsWith('merge'))))
}
