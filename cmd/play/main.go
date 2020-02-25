package main

import (
  "flag"
  "net"
  "fmt"
  "os"
  "time"

  "github.com/busoc/rt"
)

func main() {
  sleep := flag.Duration("s", time.Second, "sleep time")
  flag.Parse()

  dirs := make([]string, flag.NArg()-1)
	for i := 0; i < len(dirs); i++ {
		dirs[i] = flag.Arg(i + 1)
	}
	br, err := rt.Browse(dirs, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "browsing", err)
		os.Exit(3)
	}
  defer br.Close()

  w, err := net.Dial("udp", flag.Arg(0))
  if err != nil {
    fmt.Fprintln(os.Stderr, "dialing", err)
		os.Exit(3)
  }
  defer w.Close()

  var (
    buf = make([]byte, 8<<20)
    rs  = rt.NewReader(br)
  )
  for {
    n, err := rs.Read(buf)
    if err != nil {
      break
    }
    if _, err := w.Write(buf[:n]); err != nil {
      fmt.Fprintln(os.Stderr, "sync", err)
  		os.Exit(3)
    }
    if *sleep > 0 {
      time.Sleep(*sleep)
    }
  }
}
