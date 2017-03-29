package main

import (
    "fmt"
    "github.com/jlaffaye/ftp"
)

func main() {
  var err error
  var handle *ftp.ServerConn

  if handle, err = ftp.Connect("ftp.mozilla.org:21"); err != nil {
    panic(err)
  }
  var entries []*ftp.Entry

  if entries, err = handle.List("/pub/firefox/releases/"); err != nil {
    panic(err)
  }

  for _, entry := range entries {
    fmt.Println(entry.Name)
  }
}
