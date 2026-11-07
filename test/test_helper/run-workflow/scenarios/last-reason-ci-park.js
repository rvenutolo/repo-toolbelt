// lastReason threads each merge call's reason into the next; the merge prompt
// names the whole merge-queue file rather than a section; and a park after a
// `ci` needs-review says the worktree is ahead of the remote by an unpushed
// fix, not that it diverges, so leftovers.md does not recommend discarding it.
const L = require('../lib')

let k = 0
exports.args = L.baseArgs({ cap: 1, batch: [L.issue(1)] })
exports.respond = L.responder({
  'merge #1': () => {
    k++
    if (k === 1) {
      return L.merge({ state: 'needs-review', reason: 'conflict', fixBase: 'sha1',
        mergeWorktree: '/main/.claude/worktrees/merge-1', conflictedFiles: ['a.go'] })
    }
    if (k === 2) {
      return L.merge({ state: 'needs-review', reason: 'ci', fixBase: 'sha2',
        mergeWorktree: '/main/.claude/worktrees/merge-1', conflictedFiles: ['b.go'] })
    }
    return null
  },
  'review #1 resolution': () => L.review(),
})
exports.check = (r, t, test) => {
  test('lastReason threaded', () => {
    const calls = t.calls.filter((call) => call.label.startsWith('merge #1'))
    L.assert.deepStrictEqual(calls.map((call) => L.inputsOf(call.prompt).lastReason), ['', 'conflict', 'ci', 'ci'])
  })
  test('the merge prompt names the whole file', () => {
    const prompt = t.calls.find((call) => call.label === 'merge #1').prompt
    L.assert.match(prompt, /then follow the whole of \/main\/\.claude\/skills\/work-backlog-parallel\/merge-queue\.md\. Inputs \(JSON\): /)
    L.assert.ok(!prompt.includes('section "'))
  })
  test('the park sentence after a ci reason', () => {
    const rec = L.recOf(r, 1)
    L.assert.strictEqual(rec.state, 'parked')
    L.assert.match(rec.note, /^The merge agent returned no result twice/)
    L.assert.match(rec.note,
      /The worktree \/main\/\.claude\/worktrees\/merge-1 holds a rebased fix\/1-x that is ahead of origin\/fix\/1-x by an unpushed fix\.$/)
  })
}
