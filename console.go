package ipcheck

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	printMu        sync.Mutex
	lastWasRefresh bool
)

func consoleRefresh(format string, args ...any) {
	printMu.Lock()
	defer printMu.Unlock()
	fmt.Fprintf(os.Stdout, "\r%s\033[K", fmt.Sprintf(format, args...))
	lastWasRefresh = true
}

func consolePrint(args ...any) {
	printMu.Lock()
	defer printMu.Unlock()
	if lastWasRefresh {
		fmt.Fprint(os.Stdout, "\r\033[K")
		lastWasRefresh = false
	}
	text := strings.TrimSuffix(fmt.Sprintln(args...), "\n")
	fmt.Fprintln(os.Stdout, text)
}
