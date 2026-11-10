// A relaunch with resumeFromRunId: the first run died during merge #2, so
// every call before it returns its cached result and only merge #2 runs
// live; the second run merges both without rebuilding anything.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1), L.issue(2)] })
exports.replay = {
  firstRespond: L.responder({ 'merge #2': () => null }),
  // build #1, review #1 r1, build #2, review #2 r1, merge #1 finished; merge #2 did not.
  completed: 5,
}
exports.respond = L.responder({
  build: (label) => {
    throw new Error(`${label} re-ran live`)
  },
  review: (label) => {
    throw new Error(`${label} re-ran live`)
  },
  'merge #1': () => {
    throw new Error('merge #1 re-ran live')
  },
})
exports.check = (r, t, test) => {
  test('the first run died at merge #2', () => L.assert.ok(t.first.calls.slice(5).every((c) => c.label.startsWith('merge #2'))))
  test('only merge #2 runs live', () => {
    L.assert.deepStrictEqual(t.calls.filter((c) => !c.cached).map((c) => c.label), ['merge #2'])
  })
  test('both merge', () => L.assert.deepStrictEqual([...r.merged].sort(), [1, 2]))
}
