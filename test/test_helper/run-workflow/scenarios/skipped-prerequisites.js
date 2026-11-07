// A build that finds the deadline passed is skipped with the script's own
// fixed note (the agent's text is replaced) and its dependent skips with the
// same note; a build skipped because the user closed the issue keeps the
// agent's note and its dependent is skipped naming it. Neither dependent is
// built or parked, and no skipped build leaves a worktree in place.
const L = require('../lib')

const DEADLINE = "Not started: the run's deadline (2026-11-07T06:00:00Z) passed before this issue started."
exports.args = L.baseArgs({
  cap: 2, batch: [L.issue(1), L.issue(2), L.issue(3, { after: [1] }), L.issue(4, { after: [2] })],
})
exports.respond = L.responder({
  'build #1': () => L.writer({ state: 'skipped', deadlinePassed: true, parkNote: 'agent free text' }),
  'build #2': () => L.writer({ state: 'skipped', parkNote: 'The user closed the issue.' }),
})
exports.check = (r, t, test) => {
  test('deadline skip carries the fixed note', () => {
    L.assert.strictEqual(L.recOf(r, 1).state, 'skipped')
    L.assert.strictEqual(L.recOf(r, 1).note, DEADLINE)
  })
  test('its dependent skips with the same note', () => {
    L.assert.strictEqual(L.recOf(r, 3).state, 'skipped')
    L.assert.strictEqual(L.recOf(r, 3).note, DEADLINE)
  })
  test('a closed issue skips its dependent naming it', () => {
    L.assert.strictEqual(L.recOf(r, 4).state, 'skipped')
    L.assert.strictEqual(L.recOf(r, 4).note, 'Not started: #2 did not merge: The user closed the issue.')
  })
  test('the dependents never started', () => L.assert.deepStrictEqual(L.labels(t), ['build #1', 'build #2']))
  test('result lists', () => {
    L.assert.deepStrictEqual([...r.skipped].sort(), [1, 2, 3, 4])
    L.assert.deepStrictEqual(r.parked, [])
  })
  test('no worktree left by a skipped build', () => L.assert.deepStrictEqual(L.recOf(r, 1).leftInPlace, []))
}
