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

func fetchlist(url string) (*listing, error) {
  client := http.Client{}

  req, err := http.NewRequest(http.MethodGet, url, nil)
  if err != nil {
    return nil, err
  }
  req.Header.Set("Accept", "application/json")
  req.Header.Set("User-Agent", "systemaddons-versions")

  res, getErr := client.Do(req)
  if getErr != nil {
    return nil, getErr
  }
  body, readErr := ioutil.ReadAll(res.Body)
  if readErr != nil {
    return nil, readErr
  }

  rootList := listing{}
  if jsonErr := json.Unmarshal(body, &rootList); jsonErr != nil {
    return nil, jsonErr
  }
  return &rootList, nil
}

func main() {
  url := "http://delivery-prod-elb-1rws3domn9m17-111664144.us-west-2.elb.amazonaws.com/pub/firefox/releases/"

  releaseList, err := fetchlist(url)
  if err != nil {
    panic(err)
  }
  for _, release := range releaseList.Prefixes {
    archList, err := fetchlist(url + release)
    if err != nil {
      panic(err)
    }
    for _, arch := range archList.Prefixes {
      langList, err := fetchlist(url + release + arch)
      if err != nil {
        panic(err)
      }
      for _, lang := range langList.Prefixes {
        fileList, err := fetchlist(url + release + arch + lang)
        if err != nil {
          panic(err)
        }
        for _, file := range fileList.Files {
          fmt.Println(release + arch + lang + file.Name)
        }
      }
    }
  }
}
