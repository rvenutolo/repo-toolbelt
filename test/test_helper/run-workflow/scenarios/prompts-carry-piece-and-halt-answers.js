// An umbrella piece with halt answers: the review and fix prompts carry its
// piece and halt answers, so the reviewer judges only that piece and nobody
// re-decides an answered question, and the merge agent gets the halt answers
// and what a merge-time fix needs (kind, spec, plan). Outside the shakedown the
// merge agent's stopAtMerge is false.
const L = require('../lib')

const piece = { kind: 'feature', spec: 'docs/superpowers/specs/s.md', piece: 'first', haltAnswers: 'Q1: no' }
exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1, piece)] })
exports.respond = L.responder({
  'build #1': () => L.buildReady(1, { plan: 'docs/superpowers/plans/p.md' }),
  'review #1 r1': () => L.review({ changeFindings: [{ file: 'f', summary: 's' }] }),
})
const inputs = (t, label) => L.inputsOf(t.calls.find((call) => call.label === label).prompt)
exports.check = (r, t, test) => {
  test('merged', () => L.assert.deepStrictEqual(r.merged, [1]))
  for (const label of ['review #1 r1', 'fix #1 r1']) {
    test(`${label} carries piece and haltAnswers`, () => {
      const got = inputs(t, label)
      L.assert.strictEqual(got.piece, 'first')
      L.assert.strictEqual(got.haltAnswers, 'Q1: no')
    })
  }
  test('merge carries haltAnswers, kind, spec, plan and stopAtMerge false', () => {
    const got = inputs(t, 'merge #1')
    L.assert.strictEqual(got.haltAnswers, 'Q1: no')
    L.assert.strictEqual(got.kind, 'feature')
    L.assert.strictEqual(got.spec, 'docs/superpowers/specs/s.md')
    L.assert.strictEqual(got.plan, 'docs/superpowers/plans/p.md')
    L.assert.strictEqual(got.stopAtMerge, false)
  })
}
