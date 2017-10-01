tl [![License](http://img.shields.io/:license-agpl3-blue.svg)](http://www.gnu.org/licenses/agpl-3.0.html) [![Build Status](https://travis-ci.org/opennota/tl.png?branch=master)](https://travis-ci.org/opennota/tl)
==

![Screencast](/screencast.gif)

## Installation

    go get -u github.com/opennota/tl

Or download a pre-compiled binary from the [Releases](https://github.com/opennota/tl/releases) page.

## Usage

    tl -http :1337 -db /path/to/tl.db

- `-http :1337` - listen on port 1337 (by default port 3000 or $PORT on localhost)
- `-db /path/to/tl.db` - path to the translations database (by default `tl.db` in the current directory)

