package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/rt"
	"github.com/midbel/linewriter"
	"github.com/midbel/sizefmt"
)

const (
	OK = "\x1b[38;5;2m[ OK ]\x1b[0m"
	KO = "\x1b[38;5;1m[ KO ]\x1b[0m"
)

var buf = make([]byte, 8<<20)

type state struct {
	Packets int64
	Size    int64
	Err     error
}

type dumper struct {
	invalid bool
	csv     bool
	pretty  bool
	line    *linewriter.Writer

	Size  float64
	Lost  float64
	Files int
}

func Dump(csv, invalid, pretty bool) *dumper {
	var options []linewriter.Option
	if csv {
		options = append(options, linewriter.AsCSV(true))
	} else {
		options = []linewriter.Option{
			linewriter.WithPadding([]byte(" ")),
			linewriter.WithSeparator([]byte("|")),
		}
	}
	d := dumper{
		pretty:  pretty,
		invalid: invalid,
		csv:     csv,
		line:    linewriter.NewWriter(4096, options...),
	}
	return &d
}

func (d *dumper) WalkAndDump(file string) {
	filepath.Walk(file, func(p string, i os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if i.IsDir() {
			return nil
		}
		if s, err := checkFile(p, i); err == nil {
			d.Dump(s, i, p)
		}
		return nil
	})
}

func (d *dumper) Dump(s state, i os.FileInfo, p string) {
	missing := i.Size() - s.Size
	d.Size += float64(s.Size)
	d.Lost += float64(missing)
	d.Files++

	if d.invalid && s.Err == nil {
		return
	}

	if d.csv {
		str := "ok"
		if s.Err != nil {
			str = s.Err.Error()
		}
		d.line.AppendString(str, 12, linewriter.AlignRight)
	} else {
		str := OK
		if s.Err != nil {
			str = KO
		}
		d.line.AppendString(str, 6, linewriter.AlignLeft)
	}
	if d.csv || !d.pretty {
		d.line.AppendInt(s.Size, 13, linewriter.AlignRight|linewriter.NoSeparator)
		d.line.AppendInt(missing, 13, linewriter.AlignRight)
	} else {
		d.line.AppendSize(s.Size, 9, linewriter.AlignRight|linewriter.NoSeparator)
		d.line.AppendSize(missing, 9, linewriter.AlignRight)
	}
	d.line.AppendInt(s.Packets, 9, linewriter.AlignRight)
	d.line.AppendString(p, 0, linewriter.AlignLeft)

	io.Copy(os.Stdout, d.line)
}

func main() {
	var (
		csv     = flag.Bool("csv", false, "csv")
		invalid = flag.Bool("invalid", false, "invalid")
		pretty  = flag.Bool("pretty", false, "pretty")
	)
	flag.Parse()

	d := Dump(*csv, *invalid, *pretty)
	d.WalkAndDump(flag.Arg(0))

	fmt.Printf("%d files (size: %s, lost: %s)\n", d.Files, sizefmt.Format(d.Size, "iec"), sizefmt.Format(d.Lost, "iec"))
}

func checkFile(p string, i os.FileInfo) (state, error) {
	var s state

	r, err := os.Open(p)
	if err != nil {
		return s, err
	}
	defer r.Close()

	rs := rt.NewReader(r)
	for {
		n, err := rs.Read(buf)
		if n > 0 {
			s.Size += int64(n) + 4
		}
		if err != nil {
			if err != io.EOF {
				s.Err = err
			}
			break
		}
		s.Packets++
	}
	return s, nil
}
