// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"

	"github.com/opennota/diskv"
)

type Cache struct {
	d *diskv.Diskv
}

var cache Cache

func init() {
	ucd, err := os.UserCacheDir()
	if err != nil {
		return
	}

	d := diskv.New(diskv.Options{
		BasePath: filepath.Join(ucd, "tl"),
		Transform: func(s string) []string {
			s = fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(s)))
			return []string{s[:1], s[1:3]}
		},
		CacheSizeMax: 1024 * 1024,
		Compression:  diskv.NewGzipCompression(),
	})

	cache = Cache{d}
}

func (c Cache) Get(key string) ([]byte, error) {
	if c.d == nil {
		return nil, nil
	}

	dkey := hex.EncodeToString([]byte(key))

	return c.d.Read(dkey)
}

func (c Cache) Set(key string, data []byte) error {
	if c.d == nil {
		return nil
	}

	dkey := hex.EncodeToString([]byte(key))

	return c.d.Write(dkey, data)
}
