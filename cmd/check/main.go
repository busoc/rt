package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/busoc/rt"
	"github.com/midbel/sizefmt"
)

func main() {
	var (
		csv     = flag.Bool("csv", false, "csv")
		invalid = flag.Bool("invalid", false, "invalid")
		pretty  = flag.Bool("pretty", false, "pretty")
	)
	flag.Parse()

	d := rt.Dump(*csv, *invalid, *pretty)
	if err := d.Dump(os.Stdout, flag.Arg(0)); err == nil {
		fmt.Printf("%d files (size: %s, lost: %s)\n", d.Files, sizefmt.Format(d.Size, "iec"), sizefmt.Format(d.Lost, "iec"))
	} else {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
