// maxParked 1 at cap 2: two builds park on their own work while both hold a
// slot, so the third build, queued behind the cap, sees the spent budget when
// its slot frees and is skipped with the budget note, never started. Both park
// notes reach the records unchanged.
const L = require('../lib')

let release
const bothParked = new Promise((resolve) => (release = resolve))
let parks = 0
const park = (note) => () => {
  if (++parks === 2) release()
  return bothParked.then(() => L.writer({ state: 'parked', parkNote: note }))
}

exports.args = L.baseArgs({ cap: 2, maxParked: 1, batch: [L.issue(1), L.issue(2), L.issue(3)] })
exports.respond = L.responder({ 'build #1': park('cannot reproduce; close?'), 'build #2': park('ambiguous; which?') })
exports.check = (r, t, test) => {
  test('the first two park', () => L.assert.deepStrictEqual(r.parked.map((p) => p.n).sort(), [1, 2]))
  test('the third is skipped by the budget', () => {
    L.assert.deepStrictEqual(r.skipped, [3])
    L.assert.match(L.recOf(r, 3).note, /parked budget \(1\) was spent/)
  })
  test('the third build never started', () => L.assert.ok(!L.labels(t).includes('build #3'), L.labels(t).join(',')))
  test('park notes kept', () => L.assert.strictEqual(L.recOf(r, 1).note, 'cannot reproduce; close?'))
}
