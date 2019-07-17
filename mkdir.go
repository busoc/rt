package rt

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Packet interface {
	Pid() int
	Sid() int
	Timestamp() time.Time
}

type MakeFunc func(Packet) (string, error)

func Make(base, format string) (MakeFunc, error) {
	funcs, err := parseSpecifier(format)
	if err != nil {
		return nil, err
	}
	if len(funcs) == 0 {
		return nil, fmt.Errorf("invalid format string: %s", format)
	}
	return func(p Packet) (string, error) {
		ps := []string{base}
		for _, f := range funcs {
			p := f(p.Pid(), p.Sid(), p.Timestamp())
			if p == "" {
				continue
			}
			ps = append(ps, p)
		}
		dir := filepath.Join(ps...)
		return dir, os.MkdirAll(dir, 0755)
	}, nil
}

func parseSpecifier(str string) ([]func(int, int, time.Time) string, error) {
	var funcs []func(int, int, time.Time) string
	for i := 0; i < len(str); i++ {
		if str[i] != '%' {
			continue
		}
		i++

		var resolution int
		if isDigit(str[i]) {
			pos := i
			for isDigit(str[i]) {
				i++
			}
			x, err := strconv.Atoi(str[pos:i])
			if err != nil {
				return nil, err
			}
			resolution = x
		}

		var f func(int, int, time.Time) string
		switch str[i] {
		case 'Y':
			f = func(_, _ int, w time.Time) string { return fmt.Sprintf("%04d", w.Year()) }
		case 'M':
			f = func(_, _ int, w time.Time) string { return fmt.Sprintf("%02d", w.Month()) }
		case 'd':
			f = func(_, _ int, w time.Time) string {
				_, _, d := w.Date()
				return fmt.Sprintf("%02d", d)
			}
		case 'D':
			f = func(_, _ int, w time.Time) string { return fmt.Sprintf("%03d", w.YearDay()) }
		case 'h':
			f = func(_, _ int, w time.Time) string {
				if resolution > 0 {
					w = w.Truncate(time.Hour * time.Duration(resolution))
				}
				return fmt.Sprintf("%02d", w.Hour())
			}
		case 'm':
			f = func(_, _ int, w time.Time) string {
				if resolution > 0 {
					w = w.Truncate(time.Minute * time.Duration(resolution))
				}
				return fmt.Sprintf("%02d", w.Minute())
			}
		case 'P':
			f = func(a, _ int, w time.Time) string {
				var str string
				if a >= 0 {
					str = strconv.Itoa(a)
				}
				return str
			}
		case 'S':
			f = func(_, a int, w time.Time) string {
				var str string
				if a >= 0 {
					str = strconv.Itoa(a)
				}
				return str
			}
		default:
			return nil, fmt.Errorf("unknown specifier: %s", str[i-1:i+1])
		}
		funcs = append(funcs, f)
	}
	return funcs, nil
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
