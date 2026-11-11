#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Each test builds a fixture repo with the skills `work-backlog`, its sibling
# `work-backlog-parallel`, which inherits work-backlog's SKILL.md, and an
# unrelated `other`, and commits them, since the check reads tracked files
# only. The repo also holds a copy of the script and its data directory at the
# same depth, because the script takes its repo root from its own path. setup
# sets SERIAL and PARALLEL to the two siblings' directories.

function setup() {
  load test_helper/common
  make_repo
  SERIAL='.claude/skills/work-backlog'
  PARALLEL='.claude/skills/work-backlog-parallel'
  mkdir --parents "${SERIAL}" "${PARALLEL}" .claude/skills/other
  printf '# A\n\n## Terms\n\n## The session\n\n1. **Preconditions.** First.\n' > "${SERIAL}/SKILL.md"
  printf '# Choosing\n\n## The proposal\n\n- **Kinds:** bugs first.\n\nTo resume:\n' > "${SERIAL}/choosing.md"
  printf '# B\n\n## Ending\n' > "${PARALLEL}/SKILL.md"
  printf '# Other\n\n## Elsewhere\n' > .claude/skills/other/SKILL.md
  printf '# Readme\n\n**Recording a rise.** Raise it.\n' > README.md
  mkdir --parents .ci
  cp -- "${REPO_DIR}/.ci/check-skill-xrefs" .ci/check-skill-xrefs
  cp --recursive -- "${REPO_DIR}/.ci/skill-xrefs" .ci/skill-xrefs
}

# @description Write stdin to a file and commit every fixture file.
# @arg $1 path the file's path
function write_cited() {
  local -r path="$1"
  cat > "${path}"
  git add --all
  git -c user.name=t -c user.email=t@t commit --quiet --message fixture
}

@test 'passes when every cited name is a heading or label where it says' {
  write_cited "${PARALLEL}/notes.md" << 'EOF'
## Local

See "Terms", "Ending" and "Local", "The proposal" in `choosing.md`, "Kinds" in
`choosing.md`, "Preconditions" in `SKILL.md` and "To resume" in
`/work-backlog` `choosing.md`.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_success
  assert_output ''
  assert_stderr ''
}

@test 'binds a quote to the backticked file name just before it, on the same line or the next' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See `choosing.md` "The proposal" and then `choosing.md`
"Kinds".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_success
}

@test 'reports a name missing from the file named just before it' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See `choosing.md` "Terms".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 '.claude/skills/work-backlog/notes.md:1: "Terms" is no heading or bold label in choosing.md'
}

@test 'binds a quote to a skill-qualified file name before it' {
  write_cited "${PARALLEL}/notes.md" << 'EOF'
See `/work-backlog` `choosing.md` "To resume" and `/work-backlog` `SKILL.md` "Ending".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog-parallel/notes.md:1: "Ending" is no heading or bold label in work-backlog/SKILL.md'
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'binds a quote after a backticked skill name to that skill SKILL.md' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See `/work-backlog-parallel` "Ending" and `/work-backlog-parallel` "Terms".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog/notes.md:1: "Terms" is no heading or bold label in work-backlog-parallel/SKILL.md'
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'binds a quote after a possessive backticked skill name to that skill SKILL.md' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See `/work-backlog-parallel`'s "Ending" and `/work-backlog-parallel`'s "Terms".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog/notes.md:1: "Terms" is no heading or bold label in work-backlog-parallel/SKILL.md'
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'binds a quote after README or README.md to the repo README.md' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See README "Recording a rise", `README.md` "Recording a rise" and README "Terms".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 '.claude/skills/work-backlog/notes.md:1: "Terms" is no heading or bold label in README.md'
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'binds each quote of a list to the file named before the first' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See `choosing.md` "The proposal", "Kinds" and "To resume", or `choosing.md`
"Kinds" or "Ending".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 '.claude/skills/work-backlog/notes.md:1: "Ending" is no heading or bold label in choosing.md'
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'reads a quote after other words following a file name as bare' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
`choosing.md`'s "The proposal" and `choosing.md` then "Kinds".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line 'check-skill-xrefs: 2 dangling reference(s)'
}

@test 'reports a bare name that only another file of the skill has' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See "The proposal".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    ".claude/skills/work-backlog/notes.md:1: \"The proposal\" is no heading or bold label in this file or the SKILL.md it reads"
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'reports a bare name only the inheriting sibling SKILL.md has' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
See "Ending".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    ".claude/skills/work-backlog/notes.md:1: \"Ending\" is no heading or bold label in this file or the SKILL.md it reads"
}

@test 'reports a bare name only an unrelated skill or README.md has' {
  write_cited "${PARALLEL}/notes.md" << 'EOF'
See "Elsewhere" and "Recording a rise".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line 'check-skill-xrefs: 2 dangling reference(s)'
}

@test 'reports a name missing from the file it names, with path and line' {
  write_cited "${PARALLEL}/notes.md" << 'EOF'
# Notes

First paragraph.

Then "Ending" in `choosing.md`.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog-parallel/notes.md:5: "Ending" is no heading or bold label in choosing.md'
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'reports a name a skill-qualified file lacks even when a sibling file has it' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
"Ending" in `/work-backlog` `SKILL.md`.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog/notes.md:1: "Ending" is no heading or bold label in work-backlog/SKILL.md'
}

@test 'resolves a file name in the citing skill before the other skills' {
  printf '# B choosing\n\n## Local only\n' > "${PARALLEL}/choosing.md"
  write_cited "${PARALLEL}/notes.md" << 'EOF'
"Local only" in `choosing.md` and "The proposal" in `choosing.md`.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_success
}

@test 'reports a section-shaped bare name that exists nowhere' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
Go to "Gone section" next.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog/notes.md:1: "Gone section" is no heading or bold label in this file or the SKILL.md it reads'
}

@test 'leaves alone bare quotes that are not shaped like a section name' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
Reply "fix it now", "OK, swap", "Is this one?", "<the user's reason>", "end." and
"One two three four five six seven" are prose.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_success
}

@test 'leaves alone quoted text the script lists as prose' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
The sub-skill's "Final Review" and "Wait a minute" are not sections here.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_success
}

@test 'skips fenced code' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
```text
"Gone section" in `choosing.md`
```
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_success
}

@test 'reads a quote broken across lines whole' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
Start at "The
session" and then "Gone
section".
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog/notes.md:1: "Gone section" is no heading or bold label in this file or the SKILL.md it reads'
  assert_line --index 1 'check-skill-xrefs: 1 dangling reference(s)'
}

@test 'reports every dangling reference across files and counts them' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
"Gone one" here.
EOF
  write_cited "${PARALLEL}/notes.md" << 'EOF'
"Gone two" in `SKILL.md`.
EOF
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line \
    '.claude/skills/work-backlog/notes.md:1: "Gone one" is no heading or bold label in this file or the SKILL.md it reads'
  assert_line '.claude/skills/work-backlog-parallel/notes.md:1: "Gone two" is no heading or bold label in SKILL.md'
  assert_line 'check-skill-xrefs: 2 dangling reference(s)'
}

@test 'reads only tracked files' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
Fine.
EOF
  printf '"Gone section" here.\n' > "${SERIAL}/untracked.md"
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_success
}

@test 'checks its repo when run from a subdirectory of it' {
  write_cited "${SERIAL}/notes.md" << 'EOF'
"Gone section" here.
EOF
  cd "${SERIAL}"
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 1
  assert_line --index 0 \
    '.claude/skills/work-backlog/notes.md:1: "Gone section" is no heading or bold label in this file or the SKILL.md it reads'
}

@test 'exits 2 when no skill file is tracked' {
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs"
  assert_failure 2
  assert_stderr 'check-skill-xrefs: no tracked Markdown file under .claude/skills/<skill>/'
}

@test 'exits 2 with a usage line when given an argument' {
  run --separate-stderr "${FIXTURE_REPO}/.ci/check-skill-xrefs" extra
  assert_failure 2
  assert_stderr 'usage: check-skill-xrefs'
}
