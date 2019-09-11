package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"sync"
	"io"
	"strings"


	"github.com/midbel/toml"
	"github.com/midbel/linewriter"
	"golang.org/x/sync/errgroup"
)

type Location struct {
	Directory string
	Label     []string

	count int64
	size  int64
	first time.Time
	last  time.Time
}

type Line struct {
	line *linewriter.Writer
	mu     sync.Mutex

	csv  bool
}

func NewLine(csv bool) *Line {
	var opts []linewriter.Option
	if csv {
		opts = append(opts, linewriter.AsCSV(true))
	} else {
		opts = []linewriter.Option{
			linewriter.WithPadding([]byte(" ")),
			linewriter.WithSeparator([]byte("|")),
		}
	}
	i := Line{
		line: linewriter.NewWriter(4096, opts...),
		csv: csv,
	}
	return &i
}

func (l *Line) Dump(dir Location) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.line.AppendString(strings.Join(dir.Label, "/"), 24, linewriter.AlignLeft)
	if l.csv {
		l.line.AppendInt(dir.size, 12, linewriter.AlignRight)
	} else {
		l.line.AppendSize(dir.size, 12, linewriter.AlignRight)
	}
	l.line.AppendInt(dir.count, 12, linewriter.AlignRight)
	l.line.AppendTime(dir.first, "2006-01-02 15:04:05", linewriter.AlignRight)
	l.line.AppendTime(dir.last, "2006-01-02 15:04:05", linewriter.AlignRight)

	io.Copy(os.Stdout, l.line)
}

func main() {
	asCSV := flag.Bool("c", false, "csv")
	flag.Parse()

	c := struct{
		Base string
		Join string
		Dirs []Location `toml:"location"`
	}{}
	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()
	if err := toml.NewDecoder(r).Decode(&c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var (
		grp errgroup.Group
		line = NewLine(*asCSV)
	)
	for _, d := range c.Dirs {
		grp.Go(scanDir(line, d))
	}
	if err := grp.Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func scanDir(line *Line, dir Location) func() error {
	return func() error {
		err := filepath.Walk(dir.Directory, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if i.IsDir() {
				return nil
			}
			dir.size += i.Size()
			dir.count++
			if mod := i.ModTime(); dir.first.IsZero() || dir.first.After(mod) {
				dir.first = mod
			}
			if mod := i.ModTime(); dir.last.IsZero() || dir.last.Before(mod) {
				dir.last = mod
			}
			return nil
		})
		if err == nil {
			line.Dump(dir)
		}
		return err
	}
}
