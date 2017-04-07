package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

import (
	log "github.com/Sirupsen/logrus"
)

const VERSION string = "^[5-9][0-9]"
const TARGET string = "linux-.+"
const LANG string = "en-US"
const FILE string = "\\.tar\\.(gz|bz2)$"

type Filedesc struct {
	Name          string `json:"name"`
	Last_modified string `json:"last_modified"`
	Size          int    `json:"size"`
}

type Listing struct {
	Prefixes []string   `json:"prefixes"`
	Files    []Filedesc `json:"files"`
}

func fetchlist(url string) (*Listing, error) {
	log.WithFields(log.Fields{
		"url": url,
	}).Info("Fetch releases list")
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", USER_AGENT)

	res, getErr := client.Do(req)
	if getErr != nil {
		return nil, getErr
	}

	if res.StatusCode != 200 {
		return nil, errors.New("Could not fetch release list")
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}

	rootList := Listing{}
	if jsonErr := json.Unmarshal(body, &rootList); jsonErr != nil {
		return nil, jsonErr
	}
	return &rootList, nil
}

func WalkReleases(done <-chan struct{}, rootUrl string, minVersion string) (<-chan release, <-chan error) {
	downloads := make(chan release)
	errc := make(chan error, 1)

	if minVersion != "" {
		log.Info(fmt.Sprintf("Latest known version: %s", minVersion))
	}

	go func() {
		defer close(downloads)
		defer close(errc)

		// Nightly trunk
		trunk, err := getNightlyRelease(rootUrl, "central")
		if err != nil {
			errc <- err
		} else {
			downloads <- trunk
		}

		// Nightly Aurora
		aurora, err := getNightlyRelease(rootUrl, "aurora")
		if err != nil {
			log.Warn("Could not fetch Aurora")
		} else {
			downloads <- aurora
		}

		// Releases

		rootUrl = rootUrl + "releases/"

		versionList, err := fetchlist(rootUrl)
		if err != nil {
			errc <- err
			return
		}

		for _, versionPrefix := range versionList.Prefixes {
			version := strings.Replace(versionPrefix, "/", "", 1)
			if match, _ := regexp.MatchString(VERSION, version); !match {
				continue
			}

			if (minVersion != "") && (version <= minVersion) {
				continue
			}

			targetList, err := fetchlist(rootUrl + versionPrefix)
			if err != nil {
				errc <- err
				continue
			}
			for _, targetPrefix := range targetList.Prefixes {
				target := strings.Replace(targetPrefix, "/", "", 1)
				if match, _ := regexp.MatchString(TARGET, targetPrefix); !match {
					continue
				}
				langList, err := fetchlist(rootUrl + versionPrefix + targetPrefix)
				if err != nil {
					errc <- err
					continue
				}
				for _, langPrefix := range langList.Prefixes {
					lang := strings.Replace(langPrefix, "/", "", 1)
					if match, _ := regexp.MatchString(LANG, langPrefix); !match {
						continue
					}
					fileList, err := fetchlist(rootUrl + versionPrefix + targetPrefix + langPrefix)
					if err != nil {
						errc <- err
						continue
					}
					for _, file := range fileList.Files {
						if match, _ := regexp.MatchString(FILE, file.Name); !match {
							continue
						}
						select {
						case downloads <- release{
							Url:      rootUrl + versionPrefix + targetPrefix + langPrefix + file.Name,
							BuildID:  "unknown",
							Version:  version,
							Target:   target,
							Lang:     lang,
							Channel:  "unknown",
							Filename: file.Name}:
						case <-done:
							errc <- errors.New("walk canceled")
							return
						}
					}
				}
			}
		}
	}()
	return downloads, errc
}

func getNightlyRelease(rootUrl string, channel string) (result release, err error) {
	channelPrefix := fmt.Sprintf("nightly/latest-mozilla-%s/", channel)
	url := rootUrl + channelPrefix
	fileList, err := fetchlist(url)
	if err != nil {
		return result, err
	}

	filePattern := regexp.MustCompile(fmt.Sprintf("firefox-(.+)\\.(%s)\\.(%s)%s", LANG, TARGET, FILE))

	for _, file := range fileList.Files {
		match := filePattern.FindStringSubmatch(string(file.Name))
		if len(match) > 0 {
			version := match[1]
			lang := match[2]
			target := match[3]
			return release{
				Url:      url + file.Name,
				BuildID:  "unknown",
				Version:  version,
				Target:   target,
				Lang:     lang,
				Channel:  channel,
				Filename: file.Name}, nil
		}
	}
	return result, errors.New("Could not find nightly release")
}
