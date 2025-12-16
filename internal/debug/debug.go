package debug

import (
	"fmt"
	"os"
	"strings"
)

// IsEnabled returns true if debug mode is enabled via COSTA_DEBUG env var
func IsEnabled() bool {
	val := strings.ToLower(os.Getenv("COSTA_DEBUG"))
	return val == "1" || val == "true" || val == "yes"
}

// Printf prints debug output if COSTA_DEBUG is enabled
func Printf(format string, args ...interface{}) {
	if IsEnabled() {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format, args...)
	}
}

// Println prints debug output if COSTA_DEBUG is enabled
func Println(args ...interface{}) {
	if IsEnabled() {
		fmt.Fprint(os.Stderr, "[DEBUG] ")
		fmt.Fprintln(os.Stderr, args...)
	}
}
