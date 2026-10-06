package main

import (
	"fmt"
	"os"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/app"
	"github.com/go-sphere/sphere/core/boot"
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
	app.Execute(NewApplication)
}
