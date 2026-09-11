package main

import (
	"fmt"
	"os"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: carrel-wipe <login>")
		os.Exit(2)
	}
	login := os.Args[1]
	cfg, err := conf.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// The console command only reads an existing volume. A typo in
	// CARREL_DATA_DIR must fail, not plant a fresh server.key in the wrong
	// place.
	key, err := crypt.LoadServerKey(cfg.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st, err := store.Open(cfg.DataDir, key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mgr := session.NewManager(cfg)
	svc := user.New(st, mgr, cfg)
	link, err := svc.IssueConsoleWipeLink(login)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(link)
}
