// The build returns a finding the first review fails to record; the fix returns
// another, which the second review records. The second review clears only the
// findings it saw (the fix's), so the build's finding stays unrecorded for the
// controller instead of being lost.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1)] })
exports.respond = L.responder({
  'build #1': () => L.buildReady(1, { findings: [L.finding('bf')] }),
  'review #1 r1': () => L.review({ recordedWriterFindings: false, changeFindings: [{ file: 'a', summary: 'b' }] }),
  'fix #1 r1': () => L.writer({ worktree: '/w/f', findings: [L.finding('ff')] }),
  'review #1 r2': () => L.review({ recordedWriterFindings: true }),
})
exports.check = (r, t, test) => {
  test("the build's finding survives, the fix's is recorded", () => {
    L.assert.deepStrictEqual(L.recOf(r, 1).unrecorded.map((f) => f.symptom), ['bf'])
  })
  test('merged', () => L.assert.strictEqual(L.recOf(r, 1).state, 'merged'))
}
