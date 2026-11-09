// A build that returns ready with no PR parks without a review, and its
// dependent parks behind it.
const L = require('../lib')

exports.args = L.baseArgs({ batch: [L.issue(1), L.issue(2, { after: [1] })] })
exports.respond = L.responder({ 'build #1': () => L.buildReady(1, { pr: '' }) })
exports.check = (r, t, test) => {
  test('it parks', () => L.assert.strictEqual(L.recOf(r, 1).note, 'The build agent returned ready without a branch or PR.'))
  test('its dependent parks', () => L.assert.strictEqual(L.recOf(r, 2).note, 'Depends on #1, which ended parked.'))
  test('no review starts', () => L.assert.deepStrictEqual(L.labels(t), ['build #1']))
}
