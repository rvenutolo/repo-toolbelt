# Print `<path>\t<line>\t<name>\t<target>` for every double-quoted text in the
# Markdown files, outside fenced code, with <line> the first line of its
# paragraph. Each paragraph is read joined, so a quote broken across lines is
# read whole, and so is a file name and the quote after it. <target> names the
# file the quote cites:
#   `"Name" in `file.md``, `file.md` "Name"            -> file.md
#   `"Name" in `/skill` `file.md``, `/skill` `file.md` "Name" -> skill/file.md
#   `/skill` "Name", `/skill`'s "Name"                -> skill/SKILL.md
#   README "Name", `README.md` "Name"                 -> README.md
# A quote that follows a targeted one with only `, `, ` and `, ` or `, `, and `
# or `, or ` between them cites the same file. <target> is empty for a bare
# quote. Read by .ci/check-skill-xrefs.

function flush(   text, before, name, rest, target, part, last) {
  text = para
  last = ""
  while (match(text, /"[^"]+"/)) {
    before = substr(text, 1, RSTART - 1)
    name = substr(text, RSTART + 1, RLENGTH - 2)
    rest = substr(text, RSTART + RLENGTH)
    text = rest
    target = ""
    if (match(rest, /^ in `\/[a-z0-9-]+` `[^`]+\.md`/)) {
      split(substr(rest, 1, RLENGTH), part, "`")
      target = substr(part[2], 2) "/" part[4]
      last = ""
    } else if (match(rest, /^ in `[^`\/]+\.md`/)) {
      split(substr(rest, 1, RLENGTH), part, "`")
      target = part[2]
      last = ""
    } else {
      if (match(before, /`\/[a-z0-9-]+` `[^`\/]+\.md` $/)) {
        split(substr(before, RSTART), part, "`")
        target = substr(part[2], 2) "/" part[4]
      } else if (match(before, /`[^`\/]+\.md` $/)) {
        split(substr(before, RSTART), part, "`")
        target = part[2]
      } else if (match(before, /`\/[a-z0-9-]+`('s)? $/)) {
        split(substr(before, RSTART), part, "`")
        target = substr(part[2], 2) "/SKILL.md"
      } else if (before ~ /(^|[^A-Za-z0-9_.`\/-])README $/) {
        target = "README.md"
      } else if (last != "" && before ~ /^(, | and | or |, and |, or )$/) {
        target = last
      }
      last = target
    }
    print pfile "\t" start "\t" name "\t" target
  }
  para = ""
}
FNR == 1 { if (para != "") { flush() } fenced = 0 }
/^[[:space:]]*```/ { if (para != "") { flush() } fenced = !fenced; next }
fenced { next }
/^[[:space:]]*$/ { if (para != "") { flush() } next }
{
  line = $0
  sub(/^[[:space:]]+/, "", line)
  if (para == "") { para = line; start = FNR; pfile = FILENAME } else { para = para " " line }
}
END { if (para != "") { flush() } }
