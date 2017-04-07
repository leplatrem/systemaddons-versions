package main

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

func appMetadata(path string, info *release) (err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	re := regexp.MustCompile("BuildID=(.+)\\n.*SourceRepository=.+mozilla-(.+)\\n")
	match := re.FindStringSubmatch(string(data))
	if len(match) < 2 {
		return errors.New("Could not read metadata")
	}
	info.BuildID = match[1]
	info.Channel = match[2]
	if info.Channel == "central" {
		info.Channel = "nightly"
	}
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

func fetchUpdates(url string, release release, builtins []systemaddon) (results []systemaddon, err error) {
	// https://gecko.readthedocs.io/en/latest/toolkit/mozapps/extensions/addon-manager/SystemAddons.html#system-add-on-updates
	url = strings.Replace(url, "{VERSION}", release.Version, 1)
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
	results = make([]systemaddon, 0)
	for _, updated := range updatesList.Addons {
		results = append(results, systemaddon{updated.ID, updated.Version})
	}
	return results, nil
}
