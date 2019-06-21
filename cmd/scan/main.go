package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/busoc/rt"
)

func main() {
	flag.Parse()

	mr, err := rt.Browse(flag.Args(), true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer mr.Close()

	var (
		size int
		bad  int
		rs   = rt.NewReader(mr)
		buf  = make([]byte, 8<<20)
	)
	for i := 1; ; i++ {
		n, err := rs.Read(buf)
		switch err {
		case nil:
			size += n - 4
		case io.EOF, rt.ErrInvalid:
			fmt.Printf("%d packets (%d invalid, %dKB)\n", i-1, bad, size>>10)
			return
		default:
			i--
			bad++
		}
	}
}
