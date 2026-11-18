package policy

import (
	"fmt"
	"os"
	"sync"
)

var enterOnce sync.Once

// Enter applies the policy from ACMESH_POLICY before any command runs, once per
// process. A broken policy ends the process instead of letting it run
// unconfined: a typo in the policy file must never silently mean "no policy".
//
// Every per-command main calls this first, so that a command reached through its
// own package obeys the same rule as one reached through the table (acmesh-mq1).
func Enter() {
	enterOnce.Do(func() {
		if err := InitFromEnv(); err != nil {
			fmt.Fprintf(os.Stderr, "acmesh: %v\n", err)
			os.Exit(1)
		}
	})
}
