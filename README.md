# systemaddons-versions

An experimental view listing Firefox system-addons for each release, along their respective updated version.

## Job

This script is written in Go.

1. pull data from [Online Archives](https://archive.mozilla.org/pub/firefox/)
2. inspect archives content — namely manifests from XPI files
3. fetch potential updates from [Update server](https://gecko.readthedocs.io/en/latest/toolkit/mozapps/extensions/addon-manager/SystemAddons.html#system-add-on-updates)
4. push results to a Kinto collection

### Usage

    $ cd job/
    $ go run src/*.go

## UI

This Web application is written in Elm.

    $ cd ui/

### Setting up the development environment

    $ npm i

### Starting the dev server

    $ npm run live

### Building

    $ npm run build

### Deploying to gh-pages

    $ npm run deploy

The app should be deployed to https://leplatrem.github.io/systemaddons-versions/

## Licence

Apache2
