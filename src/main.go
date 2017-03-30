package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
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
  url := "http://delivery-prod-elb-1rws3domn9m17-111664144.us-west-2.elb.amazonaws.com/pub/firefox/releases/"

  client := http.Client{}

  req, err := http.NewRequest(http.MethodGet, url, nil)
  if err != nil {
    panic(err)
  }
  req.Header.Set("Accept", "application/json")
  req.Header.Set("User-Agent", "systemaddons-versions")

  res, getErr := client.Do(req)
  if getErr != nil {
    panic(getErr)
  }
  body, readErr := ioutil.ReadAll(res.Body)
  if readErr != nil {
    panic(readErr)
  }

  rootList := listing{}
  if err := json.Unmarshal(body, &rootList); err != nil {
    panic(err)
  }

  for _, prefix := range rootList.Prefixes {
    fmt.Println(prefix)
  }
}
