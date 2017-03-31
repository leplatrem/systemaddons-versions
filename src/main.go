package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
)

const VERSION string = "^[0-9]+"
const URL string = "http://delivery-prod-elb-1rws3domn9m17-111664144.us-west-2.elb.amazonaws.com/pub/firefox/releases/"
const OS string = "win|mac|linux"
const LANG string = "[a-z]+\\-[A-Z]+"
const FILE string = "(zip|\\d\\.tar\\.gz|dmg)$"

type filedesc struct {
	Name          string `json:"name"`
	Last_modified string `json:"last_modified"`
	Size          int    `json:"size"`
}

type listing struct {
	Prefixes []string   `json:"prefixes"`
	Files    []filedesc `json:"files"`
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
	releaseList, err := fetchlist(URL)
	if err != nil {
		panic(err)
	}

	// XXX check latest timestamp.

	for _, release := range releaseList.Prefixes {
		if match, _ := regexp.MatchString(VERSION, release); !match {
			continue
		}
		archList, err := fetchlist(URL + release)
		if err != nil {
			panic(err)
		}
		for _, arch := range archList.Prefixes {
			if match, _ := regexp.MatchString(OS, arch); !match {
				continue
			}
			langList, err := fetchlist(URL + release + arch)
			if err != nil {
				panic(err)
			}
			for _, lang := range langList.Prefixes {
				if match, _ := regexp.MatchString(LANG, lang); !match {
					continue
				}
				fileList, err := fetchlist(URL + release + arch + lang)
				if err != nil {
					panic(err)
				}
				for _, file := range fileList.Files {
					if match, _ := regexp.MatchString(FILE, file.Name); !match {
						continue
					}
					fmt.Println(release + arch + lang + file.Name)
				}
			}
		}
	}
}
