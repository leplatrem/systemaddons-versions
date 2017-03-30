package main

import (
    "encoding/json"
    "fmt"
)

type filedesc struct {
    Name string `json:"name"`
    Last_modified string `json:"last_modified"`
    Size int `json:"size"`
}

type listing struct {
    Prefixes []string `json:"prefixes"`
    Files []filedesc `json:"files"`
}

func main() {
  text := `{"prefixes":["ach/","af/","an/","ar/"],"files":[{"name":"firefox-52.0.tar.bz2","last_modified":"2017-03-06T16:24:33Z","size":58750643}]}`
  textBytes := []byte(text)

  listing1 := listing{}
  if err := json.Unmarshal(textBytes, &listing1); err != nil {
    panic(err)
  }

  for _, prefix := range listing1.Prefixes {
    fmt.Println(prefix)
  }
}
