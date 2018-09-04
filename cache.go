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
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/boltdb/bolt"
)

type Cache struct {
	db *bolt.DB
}

var cache Cache

func init() {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}

	d := filepath.Join(cacheDir, "tl")
	os.Mkdir(d, 0700)

	db, err := bolt.Open(filepath.Join(d, "cache.db"), 0600, nil)
	if err != nil {
		return
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("all"))
		return err
	}); err != nil {
		return
	}

	cache = Cache{db}
}

func (c Cache) Get(key string) ([]byte, error) {
	if c.db == nil {
		return nil, nil
	}

	var data []byte
	if err := c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("all"))
		data = b.Get([]byte(key))
		return nil
	}); err != nil {
		return nil, err
	}

	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(gzr)
}

func (c Cache) Set(key string, data []byte) error {
	if c.db == nil {
		return nil
	}

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(data); err != nil {
		return err
	}
	if err := gzw.Flush(); err != nil {
		return err
	}

	return c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("all"))
		return b.Put([]byte(key), buf.Bytes())
	})
}
