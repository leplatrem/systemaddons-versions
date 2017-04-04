package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"crypto/md5"
	"encoding/hex"
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
const AUS_URL string = "https://aus5.mozilla.org/update/3/SystemAddons/{VERSION}/{BUILD_ID}/{BUILD_TARGET}/{LOCALE}/{CHANNEL}/{OS_VERSION}/{DISTRIBUTION}/{DISTRIBUTION_VERSION}/update.xml"
const KINTO_URL string = "https://kinto.dev.mozaws.net/v1"
const KINTO_AUTH string = "Basic dXNlcjpwYXNz" // user:pass

// const VERSION string = "^[0-9]+"
// const TARGET string = "win|mac|linux"
// const LANG string = "[a-z]+\\-[A-Z]+"
// const FILE string = "(zip|\\d\\.tar\\.gz|dmg)$"
const VERSION string = "^[5-9][0-9]"
const TARGET string = "linux-x86_64"
const LANG string = "en-US"
const FILE string = "\\d\\.tar\\.(gz|bz2)$"

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
	Name    string `xml:"id"`
	Version string `xml:"version"`
}

type releaseinfo struct {
	Release  release       `json:"release"`
	Builtins []systemaddon `json:"builtins"`
	Updates  []systemaddon `json:"updates"`
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

func walkReleases(done <-chan struct{}, rootUrl string, minVersion string) (<-chan release, <-chan error) {
	downloads := make(chan release)
	errc := make(chan error, 1)

	if minVersion != "" {
		fmt.Println("Latest known version:", minVersion)
	}

	go func() {
		defer close(downloads)
		defer close(errc)

		versionList, err := fetchlist(rootUrl)
		if err != nil {
			errc <- err
			return
		}

		// XXX check and compare latest release.

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
				return
			}
			for _, targetPrefix := range targetList.Prefixes {
				target := strings.Replace(targetPrefix, "/", "", 1)
				if match, _ := regexp.MatchString(TARGET, targetPrefix); !match {
					continue
				}
				langList, err := fetchlist(rootUrl + versionPrefix + targetPrefix)
				if err != nil {
					errc <- err
					return
				}
				for _, langPrefix := range langList.Prefixes {
					lang := strings.Replace(langPrefix, "/", "", 1)
					if match, _ := regexp.MatchString(LANG, langPrefix); !match {
						continue
					}
					fileList, err := fetchlist(rootUrl + versionPrefix + targetPrefix + langPrefix)
					if err != nil {
						errc <- err
						return
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

func extract(path string, pattern string, output string) (paths []string, err error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	bz2Reader := bzip2.NewReader(reader)
	tarReader := tar.NewReader(bz2Reader)

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

		if match, _ := regexp.MatchString(pattern, header.Name); match {
			if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
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
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func appMetadata(path string, info *release) (err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	re := regexp.MustCompile("BuildID=(.+)\\n.*SourceRepository=.+/releases/mozilla-(.+)\\n")
	match := re.FindStringSubmatch(string(data))
	if len(match) < 2 {
		return errors.New("Could not read metadata")
	}
	info.BuildID = match[1]
	info.Channel = match[2]
	return nil
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
	url := strings.Replace(AUS_URL, "{VERSION}", release.Version, 1)
	url = strings.Replace(url, "{BUILD_ID}", release.BuildID, 1)
	url = strings.Replace(url, "{BUILD_TARGET}", release.Target, 1)
	url = strings.Replace(url, "{LOCALE}", release.Lang, 1)
	url = strings.Replace(url, "{CHANNEL}", release.Channel, 1)
	url = strings.Replace(url, "{OS_VERSION}", "default", 1)
	url = strings.Replace(url, "{DISTRIBUTION}", "default", 1)
	url = strings.Replace(url, "{DISTRIBUTION_VERSION}", "default", 1)

	fmt.Println("Fetch updates info", url)
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
	if res.StatusCode != 200 {
		return nil, errors.New("Could not fetch updates list")
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
		XMLName xml.Name `xml:"addon"`
		ID      string   `xml:"id,attr"`
		Version string   `xml:"version,attr"`
	}
	type updates struct {
		XMLName xml.Name `xml:"updates"`
		Addons  []addon  `xml:"addons>addon"`
	}
	updatesList := updates{}
	if err = xml.Unmarshal(body, &updatesList); err != nil {
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

			filename := filepath.Join(release.Target, release.Lang, release.Filename)
			if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
				errc <- err
				return
			}

			if _, err := os.Stat(filename); os.IsNotExist(err) {
				fmt.Println("Download", filename)
				err := download(release.Url, filename)
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

			extracted, err := extract(filename, "(application.ini|browser/features/.+\\.xpi)$", dir)
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

func lastPublish(serverUrl string) (release string, err error) {
	url := serverUrl + "/buckets/systemaddons/collections/versions/records?_sort=-release.version&_limit=1"

	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "systemaddons-versions")

	res, getErr := client.Do(req)
	if getErr != nil {
		return "", getErr
	}
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return "", readErr
	}

	type respbody struct {
		Data []releaseinfo `json:"data"`
	}
	releasesList := respbody{}
	if err = json.Unmarshal(body, &releasesList); err != nil {
		return "", err
	}
	if len(releasesList.Data) < 1 {
		return "", nil
	}
	return releasesList.Data[0].Release.Version, nil
}

func publish(serverUrl string, authHeader string, info releaseinfo) (err error) {
	hasher := md5.New()
	hasher.Write([]byte(info.Release.Url))
	recordId := hex.EncodeToString(hasher.Sum(nil))

	url := serverUrl + "/buckets/systemaddons/collections/versions/records/" + recordId

	fmt.Println("Published info to", url)

	client := http.Client{}

	type putbody struct {
		Data releaseinfo `json:"data"`
	}
	infobody := putbody{info}
	body, err := json.Marshal(infobody)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "systemaddons-versions")
	req.Header.Set("If-None-Match", "*")
	req.Header.Set("Authorization", authHeader)

	res, putErr := client.Do(req)
	if putErr != nil {
		return putErr
	}
	if (res.StatusCode != 201) && (res.StatusCode != 412) {
		return errors.New("Could not publish release info")
	}
	return nil
}

func main() {
	done := make(chan struct{})

	minVersion, err := lastPublish(KINTO_URL)
	if err != nil {
		panic(err)
	}
	releases, errc := walkReleases(done, DELIVERY_URL, minVersion)

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
		err := publish(KINTO_URL, KINTO_AUTH, r)
		if err != nil {
			panic(err)
		}
	}
	if err := <-errc; err != nil {
		panic(err)
	}
	fmt.Println("Done")
}
