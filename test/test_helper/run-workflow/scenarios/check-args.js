// checkArgs rejects a malformed run-args.json before any agent starts: a
// dependency cycle, a self-cycle, a prerequisite outside the batch, a duplicate
// issue, a feature with no spec, a deadline not in the UTC form, a string issue
// number, a string in `after`, a cap of 0 and a relative main checkout path.
// The case name is the harness's second argument.
const L = require('../lib')

const cases = {
  cycle: L.baseArgs({ batch: [L.issue(1, { after: [2] }), L.issue(2, { after: [1] })] }),
  'self-cycle': L.baseArgs({ batch: [L.issue(1, { after: [1] })] }),
  'after-outside-batch': L.baseArgs({ batch: [L.issue(1, { after: [9] })] }),
  duplicate: L.baseArgs({ batch: [L.issue(1), L.issue(1)] }),
  'feature-without-spec': L.baseArgs({ batch: [L.issue(1, { kind: 'feature' })] }),
  'bad-deadline': L.baseArgs({ deadline: '2026-11-07T06:00Z', batch: [L.issue(1)] }),
  'string-issue-number': L.baseArgs({ batch: [L.issue('1')] }),
  'string-after': L.baseArgs({ batch: [L.issue(1), L.issue(2, { after: ['1'] })] }),
  'cap-zero': L.baseArgs({ cap: 0, batch: [L.issue(1)] }),
  'relative-main': L.baseArgs({ main: 'repo', batch: [L.issue(1)] }),
}

const which = process.env.CASE
if (!cases[which]) throw new Error(`unknown check-args case ${JSON.stringify(which)}; one of ${Object.keys(cases).join(', ')}`)
exports.args = cases[which]
exports.respond = L.responder()
exports.expectThrow = (error) => L.assert.match(error.message, /^run-args.json is invalid: /)
exports.check = () => {}
