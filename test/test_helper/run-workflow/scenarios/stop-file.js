// The stop file (.claude/backlog/stop): a build agent that finds it skips with
// its note and its dependent is skipped behind it; a merge agent that finds it
// returns ready-to-merge without merging, left for the controller at Ending.
const L = require('../lib')

const note = 'stopped: .claude/backlog/stop'
exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1), L.issue(2), L.issue(3, { after: [2] })] })
exports.respond = L.responder({
  'build #2': () => L.writer({ state: 'skipped', parkNote: note, changed: false }),
  'merge #1': () => L.merge({ state: 'ready-to-merge', mergeWorktree: '/wt/1' }),
})
exports.check = (r, t, test) => {
  test('the merge that found the stop waits for the user', () => L.assert.deepStrictEqual(r.readyToMerge, [1]))
  test('the build that found the stop skips with its note', () => L.assert.strictEqual(L.recOf(r, 2).note, note))
  test('its dependent is skipped, naming why', () => {
    L.assert.deepStrictEqual([...r.skipped].sort(), [2, 3])
    L.assert.strictEqual(L.recOf(r, 3).note, `Not started: #2 did not merge: ${note}`)
  })
  test('nothing merges', () => L.assert.deepStrictEqual(r.merged, []))
}
