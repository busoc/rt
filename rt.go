package rt

import (
	// "bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var (
	ErrSkip    = errors.New("skip")
	ErrInvalid = errors.New("invalid")
)

const TimeFormat = "2006-01-02 15:04:05.000"

const Five = 5 * time.Minute

type MatchFunc func([]byte) bool

type TruncatedError int

func (e TruncatedError) Error() string {
	return fmt.Sprintf("rt: truncated %d bytes", int(e))
}

type Coze struct {
	First     uint64
	Last      uint64
	StartTime time.Time
	EndTime   time.Time

	Id      int    `json:"id"`
	Size    uint64 `json:"bytes"`
	Count   uint64 `json:"count"`
	Missing uint64 `json:"missing"`
	Error   uint64 `json:"error"`
}

func (c *Coze) Reset() {
	c.Size = 0
	c.Count = 0
	c.Error = 0
	c.Missing = 0
	c.First = 0
	c.Last = 0
	c.StartTime = time.Time{}
	c.EndTime = time.Time{}
}

func (c *Coze) Update(o *Coze) {
	c.Size += o.Size
	c.Count += o.Count
	c.Error += o.Error
	c.Missing += o.Missing
}

type Gap struct {
	Id     int       `json:"id"`
	Starts time.Time `json:"dtstart"`
	Ends   time.Time `json:"dtend"`
	Last   int       `json:"last"`
	First  int       `json:"first"`
}

func (g *Gap) Duration() time.Duration {
	return g.Ends.Sub(g.Starts)
}

func (g *Gap) Missing() int {
	d := g.First - g.Last
	if d < 0 {
		d = -d
	}
	return d - 1
}

type Offset struct {
	Pid      uint
	Time     time.Time
	Sequence uint
	Len      uint

	Cmp func(Offset, Offset) bool

	size     int
	position int64
}

func (o Offset) Less(other Offset) bool {
	if o.Cmp != nil {
		return o.Cmp(o, other)
	}
	if o.Time.Equal(other.Time) {
		if o.Pid == other.Pid {
			return o.Sequence < other.Sequence
		}
		return o.Pid < other.Pid
	}
	return o.Time.Before(other.Time)
}

type Merger struct {
	get     func([]byte) (Offset, error)
	index   []Offset
	written int64

	inner *os.File
}

func MergeFiles(files []string, w io.Writer, f func([]byte) (Offset, error)) error {
	rs := make([]io.Reader, len(files))
	for i := 0; i < len(rs); i++ {
		r, err := os.Open(files[i])
		if err != nil {
			return err
		}
		defer r.Close()
		rs[i] = r
	}
	m, err := NewMerger(f)
	if err != nil {
		return err
	}
	defer m.Close()

	buffer := make([]byte, 8<<20)
	r := NewReader(io.MultiReader(rs...))

	if _, err = io.CopyBuffer(m, r, buffer); err != nil {
		return err
	}
	if err := m.Reset(); err != nil {
		return err
	}
	_, err = io.CopyBuffer(w, m, buffer)
	return err
}

func NewMerger(get func([]byte) (Offset, error)) (*Merger, error) {
	if get == nil {
		return nil, fmt.Errorf("merger: needs a valid func")
	}
	f, err := ioutil.TempFile("", "merge_*.dat")
	if err != nil {
		return nil, err
	}
	return &Merger{inner: f, get: get}, nil
}

func (m *Merger) Reset() error {
	sort.Slice(m.index, func(i, j int) bool {
		return m.index[i].Less(m.index[j])
	})
	_, err := m.inner.Seek(0, io.SeekStart)
	return err
}

func (m *Merger) Read(bs []byte) (int, error) {
	if len(m.index) == 0 {
		return 0, io.EOF
	}
	o := m.index[0]
	if _, err := m.inner.Seek(o.position, io.SeekStart); err != nil {
		return 0, err
	}
	if len(bs) < o.size {
		return 0, io.ErrShortBuffer
	}
	n, err := m.inner.Read(bs[:o.size])
	if err == nil {
		m.index = m.index[1:]
	}
	return n, err
}

func (m *Merger) Write(bs []byte) (int, error) {
	switch o, err := m.get(bs); err {
	case nil:
		o.position, o.size = m.written, len(bs)
		m.index = append(m.index, o)
	case ErrSkip:
		return len(bs), nil
	default:
		return 0, err
	}
	n, err := m.inner.Write(bs)
	if err == nil {
		m.written += int64(n)
	}
	return n, err
}

func (m *Merger) Close() error {
	err := m.inner.Close()
	if e := os.Remove(m.inner.Name()); e != nil {
		err = e
	}
	return err
}

func Path(base string, t time.Time) (string, error) {
	t = t.Truncate(Five)

	year := fmt.Sprintf("%04d", t.Year())
	doy := fmt.Sprintf("%04d", t.YearDay())
	hour := fmt.Sprintf("%04d", t.Hour())

	dir := filepath.Join(base, year, doy, hour)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	min := t.Minute()
	file := fmt.Sprintf("rt_%02d_%02d.dat", min, min+4)
	return filepath.Join(dir, file), nil
}

type Reader struct {
	// inner *bufio.Reader
	inner io.Reader

	match  MatchFunc
	needed int
}

func NewReader(r io.Reader) *Reader {
	var rs Reader
	rs.match = func([]byte) bool { return true }
	rs.Reset(r)
	return &rs
}

func (r *Reader) Reset(rs io.Reader) {
	if rs == nil {
		return
	}
	// if r.inner == nil {
	// 	r.inner = bufio.NewReader(rs)
	// } else {
	// 	r.inner.Reset(rs)
	// }
	r.inner = rs
	r.needed = 0
}

func (r *Reader) Read(xs []byte) (int, error) {
	if r.inner == nil {
		return 0, nil
	}

	if _, err := r.inner.Read(xs[:4]); err != nil {
		return 0, err
	}

	r.needed = int(binary.LittleEndian.Uint32(xs)) + 4
	if len(xs) < r.needed {
		if d, ok := r.inner.(*multiReader); ok {
			if err := d.closeAndOpen(); err == nil {
				return r.Read(xs)
			} else {
				return 0, ErrInvalid
			}
		} else {
			return 0, ErrInvalid
		}
	}

	n, err := io.ReadFull(r.inner, xs[4:r.needed])
	if !r.match(xs[4 : 4+256]) {
		return r.Read(xs)
	}

	return n + 4, err
}

type multiReader struct {
	inner *os.File
	files <-chan string
}

func Browse(files []string, recurse bool) (io.ReadCloser, error) {
	r := multiReader{files: walk(files, recurse)}
	if f, err := r.openFile(); err != nil {
		return nil, err
	} else {
		r.inner = f
	}
	return &r, nil
}

func (m *multiReader) closeAndOpen() error {
	m.inner.Close()

	f, err := m.openFile()
	if err == nil {
		m.inner = f
	}
	return err
}

func (m *multiReader) Read(xs []byte) (int, error) {
	n, err := m.inner.Read(xs)
	if err != nil {
		if err == io.EOF {
			err = m.closeAndOpen()
			if err == nil {
				return m.Read(xs)
			}
		}
	}
	return n, err
}

func (m *multiReader) Close() error {
	return m.inner.Close()
}

func (m *multiReader) openFile() (*os.File, error) {
	f, ok := <-m.files
	if !ok {
		return nil, io.EOF
	}
	return os.Open(f)
}

func walk(files []string, recurse bool) <-chan string {
	q := make(chan string)
	go func() {
		defer close(q)
		for i := 0; i < len(files); i++ {
			filepath.Walk(files[i], func(p string, i os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if i.IsDir() {
					if !recurse {
						return filepath.SkipDir
					} else {
						return nil
					}
				}
				if e := filepath.Ext(p); e == ".dat" {
					q <- p
				}
				return nil
			})
		}
	}()
	return q
}

type writer struct {
	io.Writer
}

func NewWriter(w io.Writer) io.Writer {
	return &writer{w}
}

func (w *writer) Write(xs []byte) (int, error) {
	n := len(xs)
	if err := binary.Write(w.Writer, binary.LittleEndian, uint32(n)); err != nil {
		return 0, err
	}
	return w.Writer.Write(xs)
}

type splitWriter struct {
	closers []io.Closer
	writers []io.Writer
}

func Split(dir string, n int) (io.WriteCloser, error) {
	if n <= 1 {
		n = 2
	}
	ws := make([]io.Writer, n)
	cs := make([]io.Closer, n)
	for i := 0; i < n; i++ {
		w, err := os.Create(filepath.Join(dir, fmt.Sprintf("rt_%04d.dat", i+1)))
		if err != nil {
			return nil, err
		}
		cs[i] = w
		ws[i] = NewWriter(w)
	}
	s := splitWriter{
		writers: ws,
		closers: cs,
	}
	return &s, nil
}

func (s *splitWriter) Write(bs []byte) (int, error) {
	i := rand.Intn(len(s.writers))
	n, err := s.writers[i].Write(bs[4:])
	n += 4
	return n, err
}

func (s *splitWriter) Close() error {
	var err error
	for i := 0; i < len(s.closers); i++ {
		if e := s.closers[i].Close(); e != nil {
			err = e
		}
	}
	return err
}
