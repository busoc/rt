package rt

import (
	// "crypto/md5"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/linewriter"
	"github.com/midbel/xxh"
)

const (
	OK = "\x1b[38;5;2m[ OK ]\x1b[0m"
	KO = "\x1b[38;5;1m[ KO ]\x1b[0m"
)

type state struct {
	Packets int64
	Size    int64
	Err     error
	Sum     uint64

	File    string
	Bytes   int64
	LastMod time.Time
}

type Dumper struct {
	strip   bool
	invalid bool
	csv     bool
	pretty  bool
	line    *linewriter.Writer

	Size  float64
	Lost  float64
	Files int
}

func Dump(csv, strip, invalid, pretty bool) *Dumper {
	var options []linewriter.Option
	if csv {
		options = append(options, linewriter.AsCSV(true))
	} else {
		options = []linewriter.Option{
			linewriter.WithPadding([]byte(" ")),
			linewriter.WithSeparator([]byte("|")),
		}
	}
	d := Dumper{
		strip:   strip,
		pretty:  pretty,
		invalid: invalid,
		csv:     csv,
		line:    linewriter.NewWriter(4096, options...),
	}
	return &d
}

func (d *Dumper) Dump(w io.Writer, file string) error {
	buf := make([]byte, 8<<20)
	return filepath.Walk(file, func(p string, i os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if i.IsDir() {
			return nil
		}
		switch filepath.Ext(p) {
		case ".dat", ".bin":
		default:
			return nil
		}
		s, err := checkFile(buf, p, i)
		if err == nil {
			if d.strip {
				s.File = strings.TrimPrefix(p, file)
				if s.File == "" {
					s.File = filepath.Base(p)
				}
			}
			d.dumpState(w, s)
		}
		return err
	})
}

func (d *Dumper) dumpState(w io.Writer, s state) {
	missing := s.Bytes - s.Size
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
	d.line.AppendTime(s.LastMod, "2006-01-02 15:04:05", linewriter.AlignRight)
	d.line.AppendInt(s.Packets, 9, linewriter.AlignRight)
	d.line.AppendUint(s.Sum, 16, linewriter.AlignRight|linewriter.Hex|linewriter.WithZero)
	d.line.AppendString(s.File, 0, linewriter.AlignLeft)

	io.Copy(w, d.line)
}

func checkFile(buf []byte, p string, i os.FileInfo) (state, error) {
	var s state

	r, err := os.Open(p)
	if err != nil {
		return s, err
	}
	defer r.Close()

	s.LastMod = i.ModTime()
	s.Bytes = i.Size()
	s.File = p

	digest := xxh.New64(0)
	rs := NewReader(io.TeeReader(r, digest))
	for {
		n, err := rs.Read(buf)
		if n > 0 {
			s.Size += int64(n)
		}
		if err != nil {
			if err != io.EOF {
				s.Err = err
			}
			break
		}
		s.Packets++
	}
	s.Sum = digest.Sum64()
	return s, nil
}
