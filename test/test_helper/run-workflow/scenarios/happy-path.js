// Three issues at cap 2 with one dependency: every issue merges, the dependent
// builds only after its prerequisite merged, the cap and the serial merge hold,
// the result and record shapes match SKILL.md "The run" step 5, every prompt
// names a section that exists (or the whole merge-queue file), and each agent's
// inputs are the ones agents.md and merge-queue.md list.
const L = require('../lib')

exports.args = L.baseArgs({ cap: 2, batch: [L.issue(1), L.issue(2), L.issue(3, { after: [1] })] })
exports.respond = L.responder()
exports.check = (r, t, test) => {
  test('every issue merged', () => L.assert.deepStrictEqual([...r.merged].sort(), [1, 2, 3]))
  test('result fields', () => L.assert.deepStrictEqual(Object.keys(r).sort(),
    ['batchHalt', 'merged', 'parked', 'readyToMerge', 'records', 'skipped']))
  test('record fields', () => L.assert.deepStrictEqual(Object.keys(L.recOf(r, 1)).sort(),
    ['branch', 'leftInPlace', 'n', 'note', 'plan', 'pr', 'rulings', 'state', 'unrecorded', 'worktrees']))
  test('no batch halt', () => L.assert.strictEqual(r.batchHalt, null))
  test('the dependent builds after its prerequisite merged', () => {
    const o = t.order
    L.assert.ok(o.indexOf('end merge #1') < o.indexOf('start build #3'), o.join(','))
  })
  test('cap respected', () => L.assert.ok(t.maxRunning.buildfix <= 2, `max ${t.maxRunning.buildfix}`))
  test('merges serial', () => L.assert.ok(t.maxRunning.merge <= 1))
  test('every prompt names an existing section or the whole merge-queue file', () => {
    for (const call of t.calls) {
      const whole = L.wholeFileOf(call.prompt)
      if (whole) {
        L.assert.strictEqual(whole[1], 'merge-queue.md', 'only the merge agent gets a whole file')
        L.assert.ok(L.skillFileExists(whole[1]))
        continue
      }
      const match = L.sectionOf(call.prompt)
      L.assert.ok(match, `no section in prompt: ${call.prompt.slice(0, 200)}`)
      const [, section, file] = match
      L.assert.ok(L.skillFile(file).includes(`\n## ${section}\n`), `section "${section}" is not a ## heading of ${file}`)
    }
  })
  test('build inputs', () => {
    const inputs = L.inputsOf(t.calls.find((call) => call.label === 'build #3').prompt)
    L.assert.deepStrictEqual(Object.keys(inputs).sort(),
      ['after', 'date', 'deadline', 'haltAnswers', 'kind', 'main', 'n', 'piece', 'spec'])
  })
  test('review inputs', () => {
    const inputs = L.inputsOf(t.calls.find((call) => call.label === 'review #1 r1').prompt)
    L.assert.deepStrictEqual(Object.keys(inputs).sort(),
      ['branch', 'build', 'haltAnswers', 'headWorktree', 'kind', 'main', 'n', 'piece', 'plan', 'pr', 'round', 'spec',
        'worktrees'])
  })
  test('merge inputs', () => {
    const inputs = L.inputsOf(t.calls.find((call) => call.label === 'merge #1').prompt)
    L.assert.deepStrictEqual(Object.keys(inputs).sort(),
      ['branch', 'date', 'haltAnswers', 'kind', 'lastReason', 'main', 'n', 'pendingRulings', 'plan', 'pr',
        'resolutionFindings', 'resolutionReviewed', 'spec', 'stopAtMerge', 'worktrees'])
  })
  test('worktrees recorded', () => L.assert.deepStrictEqual(L.recOf(r, 1).worktrees, ['/main/.claude/worktrees/wf_1-1']))
}
