package main

import (
	"archive/tar"
	"compress/bzip2"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

func GetEnv(key string, byDefault string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return byDefault
	}
	return val
}

func Download(url string, output string) (err error) {
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

func Extract(path string, pattern string, output string) (paths []string, err error) {
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
