package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/busoc/rt"
)

func main() {
	datadir := flag.String("d", os.TempDir(), "data directory")
	part := flag.Int("n", 0, "part")
	flag.Parse()

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer f.Close()

	w, err := rt.Split(*datadir, *part)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer w.Close()

	_, err = io.CopyBuffer(w, rt.NewReader(f), make([]byte, 8<<20))
	if err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
