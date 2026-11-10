// Every build parks: the run parks issues until the parked budget is spent,
// skips the builds still queued with the budget note (and their dependents
// with them), merges nothing, and is not a batch halt.
const L = require('../lib')

const budget = "Not started: the run's parked budget (1) was spent."
exports.args = L.baseArgs({
  cap: 3, maxParked: 1, batch: [L.issue(1), L.issue(2), L.issue(3), L.issue(4), L.issue(5, { after: [4] })],
})
exports.respond = L.responder({ build: () => L.writer({ state: 'parked', parkNote: 'Cannot reproduce.' }) })
exports.check = (r, t, test) => {
  test('nothing merges', () => L.assert.deepStrictEqual(r.merged, []))
  test('the builds already running park with their note', () => {
    L.assert.deepStrictEqual(r.parked.map((p) => p.n).sort(), [1, 2, 3])
    for (const p of r.parked) L.assert.strictEqual(p.note, 'Cannot reproduce.')
  })
  test('the queued build and its dependent are skipped for the budget', () => {
    L.assert.deepStrictEqual([...r.skipped].sort(), [4, 5])
    L.assert.strictEqual(L.recOf(r, 4).note, budget)
    L.assert.strictEqual(L.recOf(r, 5).note, budget)
  })
  test('no batch halt', () => L.assert.strictEqual(r.batchHalt, null))
  test('no review or merge agent runs', () => L.assert.ok(L.labels(t).every((l) => l.startsWith('build'))))
}
