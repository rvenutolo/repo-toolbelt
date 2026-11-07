// A build agent that returns null is retried exactly once (a skip from
// /workflows takes two skips); a second null parks the issue with the
// "returned no result twice" note and parks its dependent as well.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 2, maxParked: 5, batch: [L.issue(1), L.issue(2, { after: [1] })] })
exports.respond = L.responder({ 'build #1': () => null })
exports.check = (r, t, test) => {
  test('one retry, then no more calls', () => L.assert.deepStrictEqual(L.labels(t), ['build #1', 'build #1 retry']))
  test('the issue parks as dead', () => {
    L.assert.strictEqual(L.recOf(r, 1).state, 'parked')
    L.assert.match(L.recOf(r, 1).note, /returned no result twice/)
  })
  test('the dependent parks naming it', () => {
    L.assert.strictEqual(L.recOf(r, 2).state, 'parked')
    L.assert.strictEqual(L.recOf(r, 2).note, 'Depends on #1, which ended parked.')
  })
  test('both in the parked list', () => L.assert.strictEqual(r.parked.length, 2))
}
