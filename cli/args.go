package cli

import (
	"strings"
)

// NormalizeArgs partitions args so that all flags (and their values) appear before positional arguments.
// This allows standard flag.FlagSet to parse flags that appear after positional arguments (e.g. `convert input.xlsx -o out.hwk`).
func NormalizeArgs(args []string, boolFlags map[string]bool) []string {
	var flags []string
	var positionals []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			// End of flags delimiter
			positionals = append(positionals, args[i+1:]...)
			break
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			// Check if flag includes value with '='
			if strings.Contains(arg, "=") {
				flags = append(flags, arg)
				i++
				continue
			}

			// Clean flag name for bool lookup (strip leading '-' or '--')
			flagName := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
			if boolFlags[flagName] || flagName == "h" || flagName == "help" {
				flags = append(flags, arg)
				i++
			} else {
				// Flag expects a value
				flags = append(flags, arg)
				if i+1 < len(args) {
					flags = append(flags, args[i+1])
					i += 2
				} else {
					i++
				}
			}
		} else {
			positionals = append(positionals, arg)
			i++
		}
	}

	return append(flags, positionals...)
}
