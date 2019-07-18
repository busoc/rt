package rt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Packet interface {
	Pid() int
	Sid() int
	Timestamp() time.Time
}

// specifications
//
// general form: %[options][A-Za-z][+offset]
//
// [x]Y[+offset]: year
// [0][x]J[+offset]: day of year (optional 0 padding)
// [0][x]M[+offset]: month (optional 0 padding)
// [0][x]D[+offset]: day of month (optional 0 padding)
// [0][x]h[+offset]: hour (optional 0 padding + truncating of x hours)
// [0][x]m[+offset]: minute (optional 0 padding + truncating of x minutes)
// [#]P: primary ID (optionally represented in hex)
// [#]S: secondary ID (optionally represented in hex)
//
// eg:
// pid  = 451
// date = 2019-07-18 10:41:23
// pattern: /storage/archives/%P/%Y/%J/%04h/rt_%05m_%05m+4.dat
// result:  /storage/archives/451/2019/199/08/rt_40_44.dat

type Builder struct {
	format  string
	version bool
	files   map[string]uint32
}

func NewBuilder(str string, version bool) (Builder, error) {
	var b Builder
	if len(str) == 0 {
		return b, fmt.Errorf("empty string")
	}
	if ix := strings.IndexByte(str, '%'); ix < 0 {
		return b, fmt.Errorf("no placeholder in string")
	}
	b.version = version
	b.format = str
	b.files = make(map[string]uint32)

	return b, nil
}

func (b *Builder) String() string {
	return b.format
}

func (b *Builder) Copy(r io.Reader, pid int, when time.Time) error {
	w, err := b.Open(pid, when)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return err
}

func (b *Builder) Open(pid int, when time.Time) (*os.File, error) {
	p, err := b.prepare(pid, when)
	if err != nil {
		return nil, err
	}

	dir, _ := filepath.Split(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	if b.version {
		if _, ok := b.files[p]; !ok {
			b.files[p]++
		}
		p = fmt.Sprintf("%s.%d", p, b.files[p])
	}

	return os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func (b *Builder) prepare(pid int, when time.Time) (string, error) {
	var (
		str strings.Builder
		f   flag
	)
	for i, z := 0, len(b.format); i < z; {
		pos := i
		for i < z && b.format[i] != '%' {
			i++
		}
		if i > pos {
			str.WriteString(b.format[pos:i])
		}
		if i >= z {
			break
		}
		i++
		if j, err := f.Parse(b.format[i:]); err != nil {
			return "", err
		} else {
			i += j
		}
		if s := f.Format(pid, when); len(s) > 0 {
			str.WriteString(s)
		}
		f.Reset()
	}
	return str.String(), nil
}

type flag struct {
	Padding  bool
	Alter    bool
	Truncate int
	Offset   int

	transform func(int, time.Time) string
}

func (f *flag) Reset() {
	f.transform = nil
	f.Padding = false
	f.Alter = false
	f.Truncate = 0
	f.Offset = 0
}

func (f *flag) Format(i int, w time.Time) string {
	return f.transform(i, w)
}

func (f *flag) Parse(str string) (int, error) {
	var (
		i      int
		size   = len(str)
		offset = true
	)
	if str[i] == '0' {
		f.Padding = true
		i++
	}
	if isDigit(str[i]) {
		pos := i
		for i < size && isDigit(str[i]) {
			i++
		}
		x, err := strconv.Atoi(str[pos:i])
		if err != nil {
			return -1, err
		}
		f.Truncate = x
	}
	if str[i] == '#' {
		i++
		f.Alter = true
	}

	switch str[i] {
	case 'P': // primary id
		f.transform, offset = f.formatID, false
	case 'S': // secondary id
		f.transform, offset = f.formatID, false
	case 'Y': // year
		f.transform, offset = f.formatYear, false
	case 'J': // day of year
		f.transform = f.formatYearDay
	case 'D': // day of month
		f.transform = f.formatDay
	case 'M': // month
		f.transform = f.formatMonth
	case 'h': // hour
		f.transform = f.formatHour
	case 'm': // minute
		f.transform = f.formatMinute
	default:
		return -1, fmt.Errorf("unsupported verb %c", str[i])
	}
	i++
	if offset && str[i] == '+' {
		i++
		pos := i
		for i < size && isDigit(str[i]) {
			i++
		}
		if i > pos {
			x, err := strconv.Atoi(str[pos:i])
			if err != nil {
				return -1, err
			}
			f.Offset = x
		} else {
			if f.Truncate > 0 {
				f.Offset = f.Truncate - 1
			}
		}
	}
	return i, nil
}

func (f *flag) formatID(i int, _ time.Time) string {
	base := 10
	if f.Alter {
		base = 16
	}
	return strconv.FormatInt(int64(i), base)
}

func (f *flag) formatYear(_ int, w time.Time) string {
	return fmt.Sprintf("%d", w.Year())
}

func (f *flag) formatYearDay(_ int, w time.Time) string {
	pattern := "%d"
	if f.Padding {
		pattern = "%03d"
	}
	return fmt.Sprintf(pattern, w.YearDay())
}

func (f *flag) formatMonth(_ int, w time.Time) string {
	if f.Alter {
		return w.Month().String()
	}
	pattern := "%d"
	if f.Padding {
		pattern = "%02d"
	}
	return fmt.Sprintf(pattern, w.Month())
}

func (f *flag) formatDay(_ int, w time.Time) string {
	pattern := "%d"
	if f.Padding {
		pattern = "%02d"
	}
	return fmt.Sprintf(pattern, w.Day())
}

func (f *flag) formatHour(_ int, w time.Time) string {
	pattern := "%d"
	if f.Padding {
		pattern = "%02d"
	}
	var (
		trunc = time.Duration(f.Truncate) * time.Hour
		off   = time.Duration(f.Offset) * time.Hour
	)
	w = truncateTime(w, trunc, off)
	return fmt.Sprintf(pattern, w.Hour())
}

func (f *flag) formatMinute(_ int, w time.Time) string {
	pattern := "%d"
	if f.Padding {
		pattern = "%02d"
	}
	var (
		trunc = time.Duration(f.Truncate) * time.Minute
		off   = time.Duration(f.Offset) * time.Minute
	)
	w = truncateTime(w, trunc, off)
	return fmt.Sprintf(pattern, w.Minute())
}

func truncateTime(w time.Time, t, o time.Duration) time.Time {
	if t > 0 {
		w = w.Truncate(t)
	}
	if o > 0 {
		w = w.Add(o)
	}
	return w
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
