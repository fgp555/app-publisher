package main

import "github.com/fgp555/app-publisher/internal/app"

// version can be overridden at build time with -ldflags.
var version = "1.0.0"

func main() {
	app.Run(version)
}
