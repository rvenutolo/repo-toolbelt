// At cap 1 the second build holds the only slot while the first issue's review
// returns findings; the fix the review prompts is queued with priority and takes
// the slot the moment the second build frees it, ahead of the third build that
// has been waiting longer. Build and fix agents together never exceed the cap.
const L = require('../lib')

let release
const reviewed = new Promise((resolve) => (release = resolve))

exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1), L.issue(2), L.issue(3)] })
exports.respond = L.responder({
  // The build holds its slot until the review has returned and the fix has asked for a slot.
  'build #2': () => reviewed.then(() => L.buildReady(2)),
  'review #1 r1': () => {
    // Two turns of the event loop: one for the review's promise to settle, one for the fix to queue.
    setImmediate(() => setImmediate(release))
    return L.review({ changeFindings: [{ file: 'a', summary: 'b' }] })
  },
})
exports.check = (r, t, test) => {
  test('every issue merged', () => L.assert.deepStrictEqual([...r.merged].sort(), [1, 2, 3]))
  test('cap never exceeded', () => L.assert.ok(t.maxRunning.buildfix <= 1, `max ${t.maxRunning.buildfix}`))
  test('the fix takes the slot ahead of the waiting build', () => {
    const o = t.order
    L.assert.ok(o.indexOf('start build #2') < o.indexOf('end review #1 r1'), o.join(','))
    L.assert.ok(o.indexOf('start fix #1 r1') < o.indexOf('start build #3'), o.join(','))
  })
}
