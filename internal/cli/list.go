package cli

import (
	"fmt"
	"io"
)

// list prints every manifest lint as "<name>\t<speed>\t<description>".
func (r Runner) list(args []string, stdout, _ io.Writer) error {
	fs := newFlagSet("list")
	if err := parseFlags(fs, args, "list", stdout); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return usagef("list takes no arguments, got %q", fs.Arg(0))
	}
	for _, lint := range r.Lints {
		if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\n", lint.Name, lint.Speed, lint.Description); err != nil {
			return fmt.Errorf("write list: %w", err)
		}
	}
	return nil
}
