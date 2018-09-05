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
	"os"
	"path/filepath"

	"github.com/opennota/dkv"
)

type Cache struct {
	d *dkv.Store
}

var cache Cache

func init() {
	ucd, err := os.UserCacheDir()
	if err != nil {
		return
	}

	d, err := dkv.New(filepath.Join(ucd, "tl"))
	if err != nil {
		return
	}

	cache = Cache{d}
}

func (c Cache) Get(key string) ([]byte, error) {
	if c.d == nil {
		return nil, nil
	}

	return c.d.Get(key)
}

func (c Cache) Put(key string, data []byte) error {
	if c.d == nil {
		return nil
	}

	return c.d.Put(key, data)
}
