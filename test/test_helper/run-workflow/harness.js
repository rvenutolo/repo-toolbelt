// Runs .claude/skills/work-backlog-parallel/run.workflow.js against a scenario
// file with the Workflow tool's globals stubbed, so the scheduler (slots, the
// serial merge queue, budgets, batch halts, dependency skips, retries) is
// tested without an agent or a GitHub call.
//
//   node harness.js <scenario.js> [case]
//
// A scenario exports `args` (the workflow args), `respond(label, prompt, opts,
// call, trace)` (what the stub agent returns for a call: a result, null, or a
// throw) and `check(result, trace, test)`, or `expectThrow(error)` when the
// script is expected to reject the args. `case` reaches the scenario as
// `process.env.CASE`. Exit 0 when every check passes, 1 when one fails, 2 when
// the harness itself breaks.
//
// A scenario that also exports `replay: { firstRespond, completed }` models a
// relaunch with resumeFromRunId: the script runs once with `firstRespond`, the
// first `completed` agent calls of that run count as finished, and the script
// runs again with `respond`, where a call whose (prompt, opts) equals a
// finished one returns that call's result without reaching `respond` (the
// Workflow tool's per-call cache). `check` sees the second run, with the
// first run's trace as `trace.first` and each cached call marked `cached`.
'use strict'
const fs = require('fs')
const path = require('path')
const assert = require('assert')

const REPO = path.resolve(__dirname, '..', '..', '..')
const SKILL = path.join(REPO, '.claude', 'skills', 'work-backlog-parallel')
const SCRIPT = path.join(SKILL, 'run.workflow.js')

// The script's `export const meta` is the first statement and ends at the
// first line that is exactly `}`; the rest is a plain async body with a
// top-level `return`, which the Workflow tool allows and node does not, hence
// the wrapper.
function loadBody() {
  const src = fs.readFileSync(SCRIPT, 'utf8')
  const metaEnd = src.indexOf('\n}\n')
  assert.ok(metaEnd > 0, 'run.workflow.js has no `export const meta` block')
  return src.slice(metaEnd + 3)
}

// The Workflow tool's cache key: an agent() call with the same prompt and
// options is the same call.
const keyOf = (prompt, opts) => JSON.stringify([prompt, opts])

async function runScript(scenario, respond = scenario.respond, cache = new Map()) {
  const trace = { calls: [], running: new Map(), maxRunning: {}, order: [], logs: [] }
  let seq = 0
  const countKind = (kind) => [...trace.running.values()].filter((v) => v === kind).length
  const stub = {
    args: scenario.args,
    log: (message) => trace.logs.push(message),
    phase: () => {},
    parallel: async (thunks) => Promise.all(thunks.map((thunk) => thunk().catch(() => null))),
    agent: async (prompt, opts = {}) => {
      const id = ++seq
      const label = opts.label || ''
      const kind = label.split(' ')[0]
      const call = { id, label, kind, prompt, opts }
      trace.calls.push(call)
      const cached = cache.get(keyOf(prompt, opts))
      if (cached && cached.length) {
        call.cached = true
        call.result = cached.shift()
        trace.order.push(`cached ${label}`)
        return call.result
      }
      trace.order.push(`start ${label}`)
      trace.running.set(id, kind)
      trace.maxRunning.buildfix = Math.max(trace.maxRunning.buildfix || 0, countKind('build') + countKind('fix'))
      trace.maxRunning.merge = Math.max(trace.maxRunning.merge || 0, countKind('merge'))
      try {
        // Yield once so concurrent starts interleave the way real agents do.
        await new Promise((resolve) => setImmediate(resolve))
        const result = await respond(label, prompt, opts, call, trace)
        call.result = result
        if (opts.schema && result && typeof result === 'object') {
          for (const key of opts.schema.required || []) {
            assert.ok(key in result, `${label}: the stub result lacks the required field ${key}`)
          }
        }
        return result
      } catch (error) {
        call.threw = String(error)
        throw error
      } finally {
        trace.running.delete(id)
        trace.order.push(`end ${label}`)
      }
    },
  }
  // eslint-disable-next-line no-new-func
  const run = new Function('args', 'log', 'phase', 'parallel', 'agent', `return (async () => {\n${loadBody()}\n})()`)
  const result = await run(stub.args, stub.log, stub.phase, stub.parallel, stub.agent)
  return { result, trace }
}

async function main() {
  const file = path.resolve(process.argv[2])
  if (process.argv[3]) process.env.CASE = process.argv[3]
  const scenario = require(file)
  const name = path.basename(file) + (process.env.CASE ? ` [${process.env.CASE}]` : '')
  let out
  try {
    if (scenario.replay) {
      const first = await runScript(scenario, scenario.replay.firstRespond)
      const cache = new Map()
      for (const call of first.trace.calls.slice(0, scenario.replay.completed)) {
        if (!('result' in call)) continue
        const key = keyOf(call.prompt, call.opts)
        cache.set(key, [...(cache.get(key) || []), call.result])
      }
      out = await runScript(scenario, scenario.respond, cache)
      out.trace.first = first.trace
    } else {
      out = await runScript(scenario)
    }
  } catch (error) {
    if (scenario.expectThrow) {
      scenario.expectThrow(error)
      console.log(`PASS ${name} (rejected: ${error.message})`)
      return
    }
    throw error
  }
  if (scenario.expectThrow) throw new Error(`expected the script to throw, got ${JSON.stringify(out.result)}`)
  const failures = []
  const test = (what, fn) => {
    try {
      fn()
    } catch (error) {
      failures.push(`${what}: ${error.message}`)
    }
  }
  scenario.check(out.result, out.trace, test)
  if (process.env.DUMP) console.log(JSON.stringify(out.result, null, 2))
  if (process.env.TRACE) console.log(out.trace.order.join('\n'))
  if (failures.length) {
    console.log(`FAIL ${name}`)
    for (const failure of failures) console.log(`  - ${failure}`)
    process.exitCode = 1
  } else {
    console.log(`PASS ${name}`)
  }
}

main().catch((error) => {
  console.error('harness error:', error)
  process.exit(2)
})
