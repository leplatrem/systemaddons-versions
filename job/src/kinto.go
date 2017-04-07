package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
)

import (
	log "github.com/Sirupsen/logrus"
)

func LastPublish(serverUrl string) (release string, err error) {
	url := serverUrl + "/buckets/systemaddons/collections/versions/records?_sort=-release.version&release.channel=beta&_limit=1"

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

	if res.StatusCode != 200 {
		return "", errors.New("Could not read remote data")
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

func Publish(serverUrl string, authHeader string, info releaseinfo) (err error) {
	hasher := md5.New()
	hasher.Write([]byte(info.Release.Url))
	recordId := hex.EncodeToString(hasher.Sum(nil))

	url := serverUrl + "/buckets/systemaddons/collections/versions/records/" + recordId

	log.WithFields(log.Fields{
		"url": url,
	}).Info("Publish release info")

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
