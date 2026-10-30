package main

import (
	"os"

	app "github.com/Khaymah/dhow/apps/cli/internal/App"
)

func main() {
	os.Exit(app.Main(os.Args[1:], app.Env{Getenv: os.Getenv, Out: os.Stdout, Err: os.Stderr}))
}
