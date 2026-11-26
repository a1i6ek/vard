// Command vard is the server (serve, the default) and its tools: migrate
// up|down|status|reset|new, admin create (the first administrator) and keys
// reseal (seal every secret again under the current key).
package main

import (
	"fmt"
	"os"
)

// version is set at build time (-ldflags "-X main.version=…", from
// `git describe`, which names the commit too).
var version = "dev"

const usage = `usage: %s [command]

  serve     run the server (the default)
  migrate   up, down or status of the database migrations; reset -yes to
            take them all down and up again; new <name> for a changelog
  admin     create the first administrator: admin create -email … -first-name …,
            with the password on standard input
  keys      reseal: seal every stored secret again under VARD_SECRET_KEY,
            opening each with it or one of VARD_OLD_SECRET_KEYS
`

func main() {
	args := os.Args[1:]
	command := "serve"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}

	switch command {
	case "serve":
		os.Exit(serveCommand())
	case "migrate":
		os.Exit(migrateCommand(args))
	case "admin":
		os.Exit(adminCommand(args))
	case "keys":
		os.Exit(keysCommand(args))
	case "help", "-h", "-help", "--help":
		fmt.Fprintf(os.Stdout, usage, os.Args[0])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", command)
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		os.Exit(2)
	}
}
