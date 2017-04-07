package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const DELIVERY_URL string = "https://archive.mozilla.org/pub/firefox/"
const AUS_URL string = "https://aus5.mozilla.org/update/3/SystemAddons/{VERSION}/{BUILD_ID}/{BUILD_TARGET}/{LOCALE}/{CHANNEL}/{OS_VERSION}/{DISTRIBUTION}/{DISTRIBUTION_VERSION}/update.xml"
const KINTO_URL string = "https://kinto-ota.dev.mozaws.net/v1"
const KINTO_AUTH string = "Basic dXNlcjpwYXNz" // user:pass

// const VERSION string = "^[0-9]+"
// const TARGET string = "win|mac|linux"
// const LANG string = "[a-z]+\\-[A-Z]+"
// const FILE string = "(zip|\\d\\.tar\\.gz|dmg)$"
// const VERSION string = "50.0b3"
const VERSION string = "^[5-9][0-9]"
const TARGET string = "linux-.+"
const LANG string = "en-US"
const FILE string = "\\.tar\\.(gz|bz2)$"

type release struct {
	Url      string `json:"url"`
	BuildID  string `json:"buildId"`
	Version  string `json:"version"`
	Target   string `json:"target"`
	Lang     string `json:"lang"`
	Channel  string `json:"channel"`
	Filename string `json:"filename"`
}

type systemaddon struct {
	Name    string `xml:"id" json:"id"`
	Version string `xml:"version" json:"version"`
}

type releaseinfo struct {
	Release  release       `json:"release"`
	Builtins []systemaddon `json:"builtins"`
	Updates  []systemaddon `json:"updates"`
}


func inspectVersions(done <-chan struct{}, releases <-chan release, results chan<- releaseinfo) <-chan error {
	errc := make(chan error, 1)

	go func() {
		defer close(errc)
		for release := range releases {

			filename := filepath.Join(release.Target, release.Lang, release.Filename)
			if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
				errc <- err
				return
			}

			if _, err := os.Stat(filename); os.IsNotExist(err) {
				fmt.Println("Download", filename)
				err := Download(release.Url, filename)
				if err != nil {
					errc <- err
					return
				}
			}

			fmt.Println("Extract release", filename)

			dir, err := ioutil.TempDir("", "systemaddons-versions")
			if err != nil {
				errc <- err
				return
			}
			defer os.RemoveAll(dir)

			extracted, err := Extract(filename, "(application.ini|browser/features/.+\\.xpi)$", dir)
			if err != nil {
				errc <- err
				return
			}

			var builtins []systemaddon
			for _, path := range extracted {
				if strings.HasSuffix(path, ".xpi") {
					fmt.Println("Inspect addon", path)
					addon, err := addonVersion(path)
					if err != nil {
						errc <- err
						return
					}
					builtins = append(builtins, *addon)
				} else {
					fmt.Println("Read build metadata", path)
					if err = appMetadata(path, &release); err != nil {
						errc <- err
						return
					}
				}
			}

			updates, err := fetchUpdates(release, builtins)
			if err != nil {
				errc <- err
				return
			}

			infos := releaseinfo{release, builtins, updates}
			select {
			case results <- infos:
			case <-done:
				errc <- errors.New("inspection canceled")
				return
			}
		}
	}()

	return errc
}

func main() {
	done := make(chan struct{})

	minVersion, err := LastPublish(KINTO_URL)
	if err != nil {
		panic(err)
	}
	releases, errc := WalkReleases(done, DELIVERY_URL, minVersion)

	results := make(chan releaseinfo)

	var wg sync.WaitGroup
	const nbWorkers = 10
	wg.Add(nbWorkers)
	for i := 0; i < nbWorkers; i++ {
		go func() {
			dlerrc := inspectVersions(done, releases, results)
			if err := <-dlerrc; err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		err := Publish(KINTO_URL, KINTO_AUTH, r)
		if err != nil {
			panic(err)
		}
	}
	if err := <-errc; err != nil {
		panic(err)
	}
	fmt.Println("Done")
}
