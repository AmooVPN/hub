package main

import "os"

func main() {
	logger := defaultLoggerFactory()
	if err := execute(os.Args[1:], logger); err != nil {
		logger.Error("command failed", "error", err)
		os.Exit(1)
	}
}
