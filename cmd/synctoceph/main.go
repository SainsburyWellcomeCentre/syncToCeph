// synctoceph copies data from a lab acquisition computer into its own folder
// on the lab's archive share, and verifies every copy with SHA-256. This file
// only starts the program; the commands are defined in internal/cli.
package main

import (
	"os"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/cli"
)

func main() {
	os.Exit(cli.Main())
}
