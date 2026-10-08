package main

import (
	"fmt"
	"os"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/app"
	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/utils/idgenerator"
)

func main() {
	// sphere no longer sets the process timezone implicitly; this template
	// pins time.Local and TZ to boot.DefaultTimezone (Asia/Shanghai). Pass
	// another IANA zone to change it, or delete this block to keep the host
	// timezone. The lookup needs tzdata on the host or a time/tzdata import.
	if err := boot.InitTimezone(boot.DefaultTimezone); err != nil {
		fmt.Fprintf(os.Stderr, "Boot error: init timezone: %v\n", err)
		os.Exit(1)
	}
	// idgenerator builds its generator on first use, so without this call a
	// malformed WORKER_ID only surfaces as a panic inside the ent DefaultFunc
	// of the first insert. Initialize it here to fail at boot instead; the
	// generated ID columns depend on it being initialized before any insert.
	if err := idgenerator.InitFromEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "Boot error: init id generator: %v\n", err)
		os.Exit(1)
	}
	app.Execute(NewApplication)
}
