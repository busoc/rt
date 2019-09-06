package rt

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PacketInfo struct {
	UPI  string
	Pid  int
	Sid  int
	When time.Time
}

type Formatter interface {
	Format(PacketInfo) string
}

func Parse(pattern string) (Formatter, error) {
	var (
		funcs  []Formatter
		offset int
	)
	for {
		ix := strings.IndexByte(pattern[offset:], '%')
		if ix == -1 {
			break
		}

		funcs = append(funcs, literal(pattern[offset:offset+ix]))
		offset += ix + 1
		if fn, n, err := parseSpecifier(pattern[offset:]); err != nil {
			return nil, err
		} else {
			offset += n
			funcs = append(funcs, fn)
		}
	}
	var f Formatter
	if len(funcs) == 0 {
		f = literal(pattern)
	} else {
		if offset > 0 {
			funcs = append(funcs, literal(pattern[offset:]))
		}
		f = formatter{
			funcs: funcs,
		}
	}
	return f, nil
}

type formatter struct {
	str   strings.Builder
	funcs []Formatter
}

func (f formatter) Format(i PacketInfo) string {
	defer f.str.Reset()
	for _, fn := range f.funcs {
		f.str.WriteString(fn.Format(i))
	}
	return strings.TrimSpace(f.str.String())
}

type literal string

func (i literal) Format(_ PacketInfo) string {
	return string(i)
}

type formatFunc func(PacketInfo) string

func (f formatFunc) Format(i PacketInfo) string {
	return f(i)
}

const (
	pad0 = 0
	pad2 = 2
	pad3 = 3
)

// syntax: %[0][+/-0]X
type specifier struct {
	add time.Duration
	sub time.Duration

	padding   int
	transform formatFunc
}

func parseSpecifier(pattern string) (Formatter, int, error) {
	var (
		offset  int
		add     int
		sub     int
		padding bool
		spec    specifier
	)
	if pattern[offset] == '0' {
		offset++
		padding = true
	}

	for {
		if isLetter(pattern[offset]) {
			break
		}
		if char := pattern[offset]; char == '-' || char == '+' {
			offset++
			if !isDigit(pattern[offset]) {
				return nil, 0, fmt.Errorf("invalid syntax: unexpected character %c (should be a digit)", pattern[offset])
			}
			pos := offset
			for isDigit(pattern[offset]) {
				offset++
			}
			n, err := strconv.Atoi(pattern[pos:offset])
			if err != nil {
				return nil, 0, err
			}
			if char == '-' {
				sub += n
			} else {
				add += n
			}
		} else {
			return nil, 0, fmt.Errorf("invalid syntax: unexpected character %c (should be - or +)", char)
		}
	}

	switch pattern[offset] {
	case 'T':
		spec.sub = time.Duration(sub) * time.Second
		spec.add = time.Duration(add) * time.Second
		spec.transform = spec.formatTimestamp
	case 'Y':
		spec.transform = spec.formatYear
	case 'D':
		if padding {
			spec.padding = pad3
		}
		spec.sub = time.Duration(sub) * time.Hour * 24
		spec.add = time.Duration(add) * time.Hour * 24
		spec.transform = spec.formatDOY
	case 'm':
		if padding {
			spec.padding = pad2
		}
		spec.transform = spec.formatMonth
	case 'd':
		if padding {
			spec.padding = pad2
		}
		spec.transform = spec.formatDay
	case 'H':
		if padding {
			spec.padding = pad2
		}
		spec.sub = time.Duration(sub) * time.Hour
		spec.add = time.Duration(add) * time.Hour
		spec.transform = spec.formatHour
	case 'M':
		if padding {
			spec.padding = pad2
		}
		spec.sub = time.Duration(sub) * time.Minute
		spec.add = time.Duration(add) * time.Minute
		spec.transform = spec.formatMinute
	case 'P':
		spec.transform = spec.formatPid
	case 'S':
		spec.transform = spec.formatSid
	case 'U':
		spec.transform = spec.formatUPI
	default:
		return nil, 0, fmt.Errorf("unknown specifier: %c", pattern[offset])
	}
	offset++
	return spec, offset, nil
}

func (s specifier) Format(i PacketInfo) string {
	return s.transform.Format(i)
}

func (s specifier) formatTimestamp(i PacketInfo) string {
	w := updateTime(i.When, s.sub, s.add)
	return formatInt(w.Unix(), pad0)
}

func (s specifier) formatYear(i PacketInfo) string {
	return formatInt(int64(i.When.Year()), pad0)
}

func (s specifier) formatDOY(i PacketInfo) string {
	w := updateTime(i.When, s.sub, s.add)
	return formatInt(int64(w.YearDay()), s.padding)
}

func (s specifier) formatMonth(i PacketInfo) string {
	return formatInt(int64(i.When.Month()), s.padding)
}

func (s specifier) formatDay(i PacketInfo) string {
	return formatInt(int64(i.When.Day()), s.padding)
}

func (s specifier) formatHour(i PacketInfo) string {
	w := updateTime(i.When, s.sub, s.add)
	return formatInt(int64(w.Hour()), s.padding)
}

func (s specifier) formatMinute(i PacketInfo) string {
	w := updateTime(i.When, s.sub, s.add)
	return formatInt(int64(w.Minute()), s.padding)
}

func (s specifier) formatPid(i PacketInfo) string {
	return formatInt(int64(i.Pid), pad0)
}

func (s specifier) formatSid(i PacketInfo) string {
	return formatInt(int64(i.Sid), pad0)
}

func (s specifier) formatUPI(i PacketInfo) string {
	return i.UPI
}

func formatInt(n int64, padding int) string {
	if padding < 0 {
		padding = 0
	}
	return fmt.Sprintf("%0[2]*[1]d", n, padding)
}

func updateTime(w time.Time, sub, add time.Duration) time.Time {
	if sub > 0 {
		w = w.Truncate(sub)
	}
	if add > 0 {
		w = w.Add(add)
	}
	return w
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
