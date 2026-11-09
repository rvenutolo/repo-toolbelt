// A merge agent that returns no result is retried once; dead twice, the issue
// parks with the dead-agent note, its dependent parks behind it, and an
// unrelated issue still merges.
const L = require('../lib')

exports.args = L.baseArgs({ batch: [L.issue(1), L.issue(2), L.issue(3, { after: [1] })] })
exports.respond = L.responder({ 'merge #1': () => null })
exports.check = (r, t, test) => {
  test('the merge agent is retried once', () => {
    L.assert.deepStrictEqual(L.labels(t).filter((l) => l.startsWith('merge #1')), ['merge #1', 'merge #1 retry'])
  })
  test('it parks with the dead-agent note', () => {
    L.assert.strictEqual(L.recOf(r, 1).note,
      'The merge agent returned no result twice (an API failure, or skipped from /workflows).')
  })
  test('its dependent parks', () => L.assert.strictEqual(L.recOf(r, 3).note, 'Depends on #1, which ended parked.'))
  test('the unrelated issue merges', () => L.assert.deepStrictEqual(r.merged, [2]))
}
