package main

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const DELIVERY_URL string = "http://delivery-prod-elb-1rws3domn9m17-111664144.us-west-2.elb.amazonaws.com/pub/firefox/releases/"
const AUS_URL string = "https://aus5.mozilla.org/update/3/SystemAddons/{VERSION}/{BUILD_ID}/{BUILD_TARGET}/{LOCALE}/}{CHANNEL}/{OS_VERSION}/{DISTRIBUTION}/{DISTRIBUTION_VERSION}/update.xml"

// const VERSION string = "^[0-9]+"
// const OS string = "win|mac|linux"
// const LANG string = "[a-z]+\\-[A-Z]+"
// const FILE string = "(zip|\\d\\.tar\\.gz|dmg)$"
const VERSION string = "^53\\.0b2"
const OS string = "linux"
const LANG string = "en-US"
const FILE string = "\\d\\.tar\\.(gz|bz2)$"

type release struct {
	Url      string
	BuildID  string
	Version  string
	Target   string
	Platform string
	Arch     string
	Lang     string
	Channel  string
	Filename string
}

type systemaddon struct {
	Name    string `xml:"id"`
	Version string `xml:"version"`
}

type releaseinfo struct {
	Release  release
	Builtins []systemaddon
	Updates  []systemaddon
}

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
	fmt.Println("Fetch releases list", url)
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

func walkReleases(done <-chan struct{}, rootUrl string) (<-chan release, <-chan error) {
	downloads := make(chan release)
	errc := make(chan error, 1)

	go func() {
		defer close(downloads)

		versionList, err := fetchlist(rootUrl)
		if err != nil {
			errc <- err
			return
		}

		// XXX check and compare latest release.

		for _, versionPrefix := range versionList.Prefixes {
			if match, _ := regexp.MatchString(VERSION, versionPrefix); !match {
				continue
			}
			platformList, err := fetchlist(rootUrl + versionPrefix)
			if err != nil {
				errc <- err
				return
			}
			for _, platformPrefix := range platformList.Prefixes {
				if match, _ := regexp.MatchString(OS, platformPrefix); !match {
					continue
				}
				langList, err := fetchlist(rootUrl + versionPrefix + platformPrefix)
				if err != nil {
					errc <- err
					return
				}
				for _, langPrefix := range langList.Prefixes {
					if match, _ := regexp.MatchString(LANG, langPrefix); !match {
						continue
					}
					fileList, err := fetchlist(rootUrl + versionPrefix + platformPrefix + langPrefix)
					if err != nil {
						errc <- err
						return
					}
					for _, file := range fileList.Files {
						if match, _ := regexp.MatchString(FILE, file.Name); !match {
							continue
						}

						// Prepare release attributes.
						url := rootUrl + versionPrefix + platformPrefix + langPrefix + file.Name
						version := strings.Replace(versionPrefix, "/", "", 1)
						lang := strings.Replace(langPrefix, "/", "", 1)
						// linux-i686/ becomes [linux, i686]
						target := strings.Replace(platformPrefix, "/", "", 1)
						osArch := strings.SplitN(target, "-", 2)
						os := osArch[0]
						arch := osArch[len(osArch)-1]

						select {
						case downloads <- release{url, "", version, target, os, arch, lang, "", file.Name}:
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

func download(url string, output string) (err error) {
	out, err := os.Create(output + ".part")
	defer out.Close()
	if err != nil {
		return err
	}

	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	err = os.Rename(output+".part", output)
	if err != nil {
		return err
	}

	return nil
}

func extract(path string, output string) (paths []string, err error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	bz2Reader := bzip2.NewReader(reader)
	tarReader := tar.NewReader(bz2Reader)

	var results []string
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if header == nil {
			break
		}
		path := filepath.Join(output, header.Name)
		info := header.FileInfo()

		if match, _ := regexp.MatchString("browser/features/.+\\.xpi", header.Name); match {
			if err = os.MkdirAll(filepath.Dir(path), 0777); err != nil {
				return nil, err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if err != nil {
				return nil, err
			}
			defer file.Close()
			_, err = io.Copy(file, tarReader)
			if err != nil {
				return nil, err
			}
			results = append(results, path)
		}
	}
	return results, nil
}

func addonVersion(path string) (result *systemaddon, err error) {
	// Extract XPI (as Zip)
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	//
	// <RDF xmlns="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	//      xmlns:em="http://www.mozilla.org/2004/em-rdf#">

	//   <Description about="urn:mozilla:install-manifest">
	//     <em:id>aushelper@mozilla.org</em:id>
	//     <em:version>2.0</em:version>
	//
	type systemaddonrdf struct {
		XMLName     xml.Name    `xml:"RDF"`
		Description systemaddon `xml:"Description"`
	}

	// Inspect version from manifest
	for _, f := range r.File {
		if f.Name == "install.rdf" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			// Read RDF data
			data, err := ioutil.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			// Parse XML
			result := systemaddonrdf{}
			if err = xml.Unmarshal(data, &result); err != nil {
				return nil, err
			}
			return &result.Description, nil
		}
	}
	return nil, errors.New("Cannot find install.rdf")
}

func fetchUpdates(release release, builtins []systemaddon) (results []systemaddon, err error) {
	// https://gecko.readthedocs.io/en/latest/toolkit/mozapps/extensions/addon-manager/SystemAddons.html#system-add-on-updates
	// https://aus5.mozilla.org/update/3/SystemAddons/{VERSION}/{BUILD_ID}/{BUILD_TARGET}/{LOCALE}/}{CHANNEL}/{OS_VERSION}/{DISTRIBUTION}/{DISTRIBUTION_VERSION}/update.xml"
	// https://aus5.mozilla.org/update/3/Firefox/29.0a2/20140312004001/Linux_x86-gcc3/sr/aurora/default/default/default/update.xml

	url := strings.Replace(AUS_URL, "{VERSION}", release.Version, 1)
	url = strings.Replace(url, "{BUILD_ID}", release.BuildID, 1)
	url = strings.Replace(url, "{BUILD_TARGET}", release.Target, 1)
	url = strings.Replace(url, "{LOCALE}", release.Lang, 1)
	url = strings.Replace(url, "{CHANNEL}", release.Channel, 1)
	url = strings.Replace(url, "{OS_VERSION}", "default", 1)
	url = strings.Replace(url, "{DISTRIBUTION}", "default", 1)
	url = strings.Replace(url, "{DISTRIBUTION_VERSION}", "default", 1)

	// VERSION
	//     Firefox version number
	// BUILD_ID
	//     Build ID
	// BUILD_TARGET
	//     Build target
	// LOCALE
	//     Build locale
	// CHANNEL
	//     Update channel
	// OS_VERSION
	//     OS Version
	// DISTRIBUTION
	//     Firefox Distribution
	// DISTRIBUTION_VERSION
	//     Firefox Distribution version

	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/xml")
	req.Header.Set("User-Agent", "systemaddons-versions")

	res, getErr := client.Do(req)
	if getErr != nil {
		return nil, getErr
	}
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}

	// <?xml version="1.0"?>
	// <updates>
	//   <addons>
	//     <addon id="flyweb@mozilla.org" URL="https://ftp.mozilla.org/pub/system-addons/flyweb/flyweb@mozilla.org-1.0.xpi" hashFunction="sha512" hashValue="abcdef123" size="1234" version="1.0"/>
	//     <addon id="pocket@mozilla.org" URL="https://ftp.mozilla.org/pub/system-addons/pocket/pocket@mozilla.org-1.0.xpi" hashFunction="sha512" hashValue="abcdef123" size="1234" version="1.0"/>
	//   </addons>
	// </updates>
	type addon struct {
		ID      string `xml:"id,attr"`
		Version string `xml:"version,attr"`
	}
	type updates struct {
		XMLName xml.Name `xml:"updates"`
		Addons  []addon  `xml:"addon"`
	}
	updatesList := updates{}
	if err = json.Unmarshal(body, &updatesList); err != nil {
		return nil, err
	}
	for _, updated := range updatesList.Addons {
		results = append(results, systemaddon{updated.ID, updated.Version})
	}
	return results, nil
}

func inspectVersions(done <-chan struct{}, releases <-chan release, results chan<- releaseinfo) <-chan error {
	errc := make(chan error, 1)

	go func() {
		defer close(errc)
		for release := range releases {
			fmt.Println("Inspect release", release)

			filename := release.Platform + "-" + release.Arch + "-" + release.Lang + "-" + release.Filename

			if _, err := os.Stat(filename); os.IsNotExist(err) {
				fmt.Println("Download", filename)
				err := download(release.Url, filename)
				if err != nil {
					errc <- err
				}
			}

			dest := release.Platform + "-" + release.Arch + "-" + release.Lang + "-" + release.Version
			if err := os.MkdirAll(dest, 0755); err != nil {
				errc <- err
			}
			addons, err := extract(filename, dest)
			if err != nil {
				errc <- err
			}

			var builtins []systemaddon
			for _, addon := range addons {
				fmt.Println("Inspect addon", addon)
				addon, err := addonVersion(addon)
				if err != nil {
					errc <- err
				}
				builtins = append(builtins, *addon)
			}

			// XXX: remove dir

			updates, err := fetchUpdates(release, builtins)
			if err != nil {
				errc <- err
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
	defer close(done)

	releases, errc := walkReleases(done, DELIVERY_URL)

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
		fmt.Println("Done", r)
	}
	if err := <-errc; err != nil {
		panic(err)
	}
}
