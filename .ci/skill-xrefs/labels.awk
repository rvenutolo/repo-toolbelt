# Print `<path>\t<label>` for every Markdown heading, bold label (`**Name**`,
# `**Name:**`, `**Name.**`, quotes dropped) and `Name:` line, outside fenced
# code. Read by .ci/check-skill-xrefs.

FNR == 1 { fenced = 0 }
/^[[:space:]]*```/ { fenced = !fenced; next }
fenced { next }
/^#+[[:space:]]/ {
  label = $0
  sub(/^#+[[:space:]]+/, "", label)
  sub(/[[:space:]]+$/, "", label)
  print FILENAME "\t" label
}
/^[A-Z][^:`*]*:[[:space:]]*$/ {
  label = $0
  sub(/:[[:space:]]*$/, "", label)
  print FILENAME "\t" label
}
{
  line = $0
  while (match(line, /\*\*[^*]+\*\*/)) {
    label = substr(line, RSTART + 2, RLENGTH - 4)
    sub(/[:.]$/, "", label)
    gsub(/^"|"$/, "", label)
    print FILENAME "\t" label
    line = substr(line, RSTART + RLENGTH)
  }
}
