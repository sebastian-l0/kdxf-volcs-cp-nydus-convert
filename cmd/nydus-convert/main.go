package main

import (
	"context"
	"os"

	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/cli"
)

func main() {
	os.Exit(cli.Main(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
