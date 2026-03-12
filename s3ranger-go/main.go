package main

import (
	"os"

	"github.com/s3ranger/s3ranger-go/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
