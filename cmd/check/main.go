package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/busoc/rt"
)

func main() {
	var (
		strip   = flag.Bool("strip", false, "strip")
		csv     = flag.Bool("csv", false, "csv")
		invalid = flag.Bool("invalid", false, "invalid")
		pretty  = flag.Bool("pretty", false, "pretty")
	)
	flag.Parse()

	d := rt.Dump(*csv, *strip, *invalid, *pretty)
	if err := d.Dump(os.Stdout, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
