// Two review rounds that both find changes are each followed by a fix; there
// is no third review, and the issue goes to the merge queue.
const L = require('../lib')

exports.args = L.baseArgs({ batch: [L.issue(1)] })
exports.respond = L.responder({ review: () => L.review({ changeFindings: [{ file: 'a', summary: 'rename x' }] }) })
exports.check = (r, t, test) => {
  test('two rounds, then the merge', () => {
    L.assert.deepStrictEqual(L.labels(t), ['build #1', 'review #1 r1', 'fix #1 r1', 'review #1 r2', 'fix #1 r2', 'merge #1'])
  })
  test('it merges', () => L.assert.deepStrictEqual(r.merged, [1]))
}
