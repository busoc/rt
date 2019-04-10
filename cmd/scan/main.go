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

	buffer := make([]byte, 8<<20)

	var size, invalid int
	for i := 1; ; i++ {
		n, err := mr.Read(buffer)
		if n > 0 {
			size += n - 4
		}
		switch err {
		case nil:
		case io.EOF:
			fmt.Printf("%d packets (%d invalid, %dKB)\n", i-1, invalid, size>>10)
			return
		default:
			i--
			invalid++
		}
	}
}
