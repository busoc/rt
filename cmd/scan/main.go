package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/busoc/rt"
	"github.com/midbel/xxh"
)

func main() {
	list := flag.Bool("l", false, "list")
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
		sum  = xxh.New64(0)
		buf  = make([]byte, 8<<20)
		rs   = io.TeeReader(rt.NewReader(mr), sum)
	)
	for i := 1; ; i++ {
		n, err := rs.Read(buf)
		switch err {
		case nil:
			if *list {
				fmt.Printf("%7d | %7d | %016x\n", i, n, sum.Sum64())
			}
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
