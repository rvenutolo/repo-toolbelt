// A resolution review dead twice after a CI fix parks the issue with the
// "ahead of ... by an unpushed fix" sentence, and the queue moves on.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 1, maxParked: 1, batch: [L.issue(1), L.issue(2), L.issue(3)] })
exports.respond = L.responder({
  'merge #1': () => L.merge({ state: 'needs-review', reason: 'ci', mergeWorktree: '/wt/1', fixBase: 'a' }),
  'review #1 resolution': () => null,
})
exports.check = (r, t, test) => {
  test('the resolution review is retried once', () => {
    L.assert.deepStrictEqual(L.labels(t).filter((l) => l.startsWith('review #1 resolution')),
      ['review #1 resolution a1', 'review #1 resolution a1 retry'])
  })
  test('it parks with the ahead-of sentence', () => {
    L.assert.strictEqual(L.recOf(r, 1).note, 'The resolution review agent returned no result twice (an API failure, ' +
      'or skipped from /workflows). The worktree /wt/1 holds a rebased fix/1-x that is ahead of origin/fix/1-x by an ' +
      'unpushed fix.')
  })
  test('the others merge', () => L.assert.deepStrictEqual([...r.merged].sort(), [2, 3]))
}
