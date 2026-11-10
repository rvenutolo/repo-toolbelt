// Builders for scenario files: workflow args, the agents' structured outputs
// with every required field present, a default responder (build ready, clean
// review, merged) that a scenario overrides by label, and readers of the
// result and the trace.
'use strict'
const assert = require('assert')
const fs = require('fs')
const path = require('path')

const SKILL = path.resolve(__dirname, '..', '..', '..', '.claude', 'skills', 'work-backlog-parallel')

const baseArgs = (over = {}) => ({
  date: '2026-11-06', main: '/main', cap: 2, deadline: '2026-11-07T06:00:00Z', maxParked: 2, stopBeforeMerge: false,
  batch: [],
  ...over,
})
const issue = (n, over = {}) => ({ n, kind: 'bug', after: [], spec: '', piece: '', haltAnswers: '', ...over })

const writer = (over = {}) => ({
  state: 'ready', branch: '', pr: '', plan: '', worktree: '', rulings: [], findings: [], parkNote: '', batchHalt: '',
  changed: true, deadlinePassed: false, ...over,
})
const buildReady = (n, over = {}) => writer({
  branch: `fix/${n}-x`, pr: `${100 + n}`, plan: '', worktree: `/main/.claude/worktrees/wf_1-${n}`, ...over,
})
const review = (over = {}) => ({
  changeFindings: [], otherFindings: [], batchHalt: '', recordedWriterFindings: true, ...over,
})
const merge = (over = {}) => ({
  state: 'merged', parkNote: '', batchHalt: '', mergeWorktree: '', conflictedFiles: [], leftInPlace: [], rulings: [],
  findings: [], reason: '', failure: '', fixBase: '', mergeRetries: 0, ...over,
})
const finding = (symptom) => ({ symptom, file: 'f', trigger: 't', whyReal: 'w', foundWhile: 'issue 1' })

// A responder answers by the first map key the label equals or starts with, else by role.
function responder(map = {}) {
  return (label, prompt, opts, call, trace) => {
    for (const key of Object.keys(map)) {
      if (label === key || label.startsWith(key)) return map[key](label, prompt, opts, call, trace)
    }
    const kind = label.split(' ')[0]
    const n = Number(label.match(/#(\d+)/)[1])
    if (kind === 'build') return buildReady(n)
    if (kind === 'review') return review()
    if (kind === 'fix') return buildReady(n, { worktree: `/main/.claude/worktrees/wf_1-fix${n}`, branch: '', pr: '' })
    if (kind === 'merge') return merge()
    throw new Error(`unknown label ${label}`)
  }
}

const recOf = (result, n) => result.records.find((record) => record.n === n)
const labels = (trace) => trace.calls.map((call) => call.label)
const sectionOf = (prompt) => prompt.match(/section "([^"]+)" of \S+\/(\S+)\./)
const wholeFileOf = (prompt) => prompt.match(/follow the whole of \S+\/(\S+)\. Inputs/)
const inputsOf = (prompt) => JSON.parse(prompt.slice(prompt.indexOf('Inputs (JSON): ') + 'Inputs (JSON): '.length))
const skillFile = (name) => fs.readFileSync(path.join(SKILL, name), 'utf8')
const skillFileExists = (name) => fs.existsSync(path.join(SKILL, name))

module.exports = {
  assert, baseArgs, issue, writer, buildReady, review, merge, finding, responder, recOf, labels, sectionOf,
  wholeFileOf, inputsOf, skillFile, skillFileExists,
}
