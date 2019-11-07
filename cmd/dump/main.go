package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/busoc/rt"
	"github.com/midbel/dissect"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	defer r.Close()

	d, err := dissect.NewDecoder(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "decoding", err)
		os.Exit(5)
	}

	dirs := make([]string, flag.NArg()-1)
	for i := 0; i < len(dirs); i++ {
		dirs[i] = flag.Arg(i + 1)
	}
	br, err := rt.Browse(dirs, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "browsing", err)
		os.Exit(3)
	}
	defer r.Close()

	var (
		buf    = make([]byte, 8<<20)
		rs     = rt.NewReader(br)
		count  int
		size   int
		values int
	)
	for {
		switch n, err := rs.Read(buf); err {
		case nil:
			x, err := decodeBytes(d, buf[:n])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				values += x
				size += n
				count++
			}
		case io.EOF:
			fmt.Println()
			fmt.Printf("packets: %d, values: %d (size: %dKB)", count, values, size>>10)
			fmt.Println()
			return
		default:
			fmt.Fprintln(os.Stderr, "reading:", err)
			os.Exit(6)
		}
	}
}

func decodeBytes(d *dissect.Decoder, buf []byte) (int, error) {
	values, err := d.Decode(buf)
	if err != nil {
		return 0, err
	}
	for _, v := range values {
		var (
			raw interface{}
			eng interface{}
			pos int
		)
		switch v := v.(type) {
		case *dissect.Int:
			raw, pos, eng = v.Raw, v.Pos, inspect(v.Meta)
		case *dissect.Uint:
			raw, pos, eng = v.Raw, v.Pos, inspect(v.Meta)
		case *dissect.Boolean:
			raw, pos, eng = v.Raw, v.Pos, inspect(v.Meta)
		case *dissect.Real:
			raw, pos, eng = v.Raw, v.Pos, inspect(v.Meta)
		case *dissect.Bytes:
			raw, pos = hex.EncodeToString(v.Raw), v.Pos
			eng = raw
		case *dissect.String:
			raw, pos = v.Raw, v.Pos
			eng = raw
		}
		fmt.Printf("%6d |  %16s | %32v | %32v\n", pos, v, raw, eng)
	}
	return len(values), nil
}

func inspect(m dissect.Meta) interface{} {
	switch v := m.Eng.(type) {
	case *dissect.Int:
		return v.Raw
	case *dissect.Uint:
		return v.Raw
	case *dissect.Real:
		return v.Raw
	case *dissect.String:
		return v.Raw
	case *dissect.Bytes:
	default:
	}
	return nil
}
