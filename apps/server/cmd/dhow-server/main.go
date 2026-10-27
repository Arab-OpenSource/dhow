package main

import (
	"fmt"
	"os"

	app "github.com/Arab-OpenSource/dhow/apps/server/internal/App"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "dhow-server:", err)
		os.Exit(1)
	}
}
