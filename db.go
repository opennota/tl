// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General
// Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boltdb/bolt"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInconsistent = errors.New("database is inconsistent")

	rWord = regexp.MustCompile(`\w+`)
)

type DB struct {
	*bolt.DB
}

type Book struct {
	ID                  uint64    `json:"id"`
	Title               string    `json:"title"`
	Created             time.Time `json:"created"`
	FragmentsTotal      int       `json:"fragments_total"`
	FragmentsTranslated int       `json:"fragments_translated"`
	FragmentsIDs        []uint64  `json:"fragments_ids"`
	LastActivity        time.Time `json:"last_activity"`

	Fragments []Fragment `json:"-"`
}

type Fragment struct {
	ID          uint64    `json:"id"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	Text        string    `json:"text"`
	Comment     string    `json:"comment"`
	Starred     bool      `json:"starred"`
	VersionsIDs []uint64  `json:"versions_ids"`

	Versions []TranslationVersion `json:"-"`
}

type TranslationVersion struct {
	ID      uint64    `json:"id"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Text    string    `json:"text"`
}

type StarredFragment struct {
	Created    time.Time `json:"created"`
	BookID     uint64    `json:"book_id"`
	BookTitle  string    `json:"book_title"`
	FragmentID uint64    `json:"fragment_id"`
	Fragment   string    `json:"fragment"`
}

func encode(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func marshal(b *bolt.Bucket, key uint64, val interface{}) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return b.Put(encode(key), data)
}

func unmarshal(b *bolt.Bucket, key uint64, val interface{}) (bool, error) {
	data := b.Get(encode(key))
	if data == nil {
		return false, nil
	}
	if err := json.Unmarshal(data, val); err != nil {
		return true, err
	}
	return true, nil
}

func OpenDatabase(path string, mode os.FileMode, options *bolt.Options) (DB, error) {
	db, err := bolt.Open(path, mode, options)
	if err != nil {
		return DB{}, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("index"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("fragments"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("versions"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("starred"))
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return DB{}, err
	}
	return DB{db}, nil
}

type booksByLastActivity []Book

func (t booksByLastActivity) Len() int { return len(t) }
func (t booksByLastActivity) Less(i, j int) bool {
	if a, b := t[i].LastActivity, t[j].LastActivity; a != b {
		return b.Before(a)
	}
	return t[i].ID > t[j].ID
}
func (t booksByLastActivity) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (db *DB) Books() ([]Book, error) {
	var books []Book
	err := db.View(func(tx *bolt.Tx) error {
		books = books[:0]
		b := tx.Bucket([]byte("index"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var book Book
			if err := json.Unmarshal(v, &book); err != nil {
				return err
			}
			books = append(books, book)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return books, nil
}

func (db *DB) BooksByActivity() ([]Book, error) {
	books, err := db.Books()
	if err != nil {
		return nil, err
	}
	sort.Sort(booksByLastActivity(books))
	return books, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type filterKind int

const (
	fNone filterKind = iota
	fUntranslated
	fCommented
	fStarred
	fWithTwoOrMoreVersions
	fOriginalContains
	fTranslationContains
	fOriginalLength
)

func wordCount(s string) int {
	return len(rWord.FindAllStringIndex(s, -1))
}

func (db *DB) BookWithTranslations(bid uint64, from, size int, filter filterKind, filterArg ...string) (Book, error) {
	var book Book
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}

		if size == -1 {
			size = len(book.FragmentsIDs)
		}

		if filter != fNone && filter != fTranslationContains {
			fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
			filtered := book.FragmentsIDs[:0]

			switch filter {
			case fUntranslated:
				needle := []byte(`"versions_ids":[]`)
				for _, fid := range book.FragmentsIDs {
					data := fb.Get(encode(fid))
					if !bytes.Contains(data, needle) {
						continue
					}
					filtered = append(filtered, fid)
				}
			case fCommented:
				needle := []byte(`"comment":""`)
				for _, fid := range book.FragmentsIDs {
					data := fb.Get(encode(fid))
					if bytes.Contains(data, needle) {
						continue
					}
					filtered = append(filtered, fid)
				}
			case fStarred:
				needle := []byte(`"starred":true`)
				for _, fid := range book.FragmentsIDs {
					data := fb.Get(encode(fid))
					if !bytes.Contains(data, needle) {
						continue
					}
					filtered = append(filtered, fid)
				}
			case fWithTwoOrMoreVersions:
				for _, fid := range book.FragmentsIDs {
					data := fb.Get(encode(fid))
					if data == nil {
						continue
					}
					var tmp struct {
						IDs []uint64 `json:"versions_ids"`
					}
					if err := json.Unmarshal(data, &tmp); err != nil {
						return err
					}
					if len(tmp.IDs) < 2 {
						continue
					}
					filtered = append(filtered, fid)
				}
			case fOriginalContains:
				for _, fid := range book.FragmentsIDs {
					data := fb.Get(encode(fid))
					if data == nil {
						continue
					}
					var tmp struct{ Text string }
					if err := json.Unmarshal(data, &tmp); err != nil {
						return err
					}
					if !strings.Contains(tmp.Text, filterArg[0]) {
						continue
					}
					filtered = append(filtered, fid)
				}
			case fOriginalLength:
				compare := func(a, b int) bool { return a < b }
				if filterArg[0] == "more" {
					compare = func(a, b int) bool { return a > b }
				}
				n, _ := strconv.Atoi(filterArg[1])
				count := utf8.RuneCountInString
				if filterArg[2] == "words" {
					count = wordCount
				}
				for _, fid := range book.FragmentsIDs {
					data := fb.Get(encode(fid))
					if data == nil {
						continue
					}
					var tmp struct{ Text string }
					if err := json.Unmarshal(data, &tmp); err != nil {
						return err
					}
					if !compare(count(tmp.Text), n) {
						continue
					}
					filtered = append(filtered, fid)
				}
			}

			book.FragmentsIDs = filtered
		}

		if from >= len(book.FragmentsIDs) {
			return nil
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		vb := tx.Bucket([]byte("versions")).Bucket(encode(bid))
		to := min(len(book.FragmentsIDs), from+size)
		for _, fid := range book.FragmentsIDs[from:to] {
			var f Fragment
			if _, err := unmarshal(fb, fid, &f); err != nil {
				return err
			}

			if filter != fUntranslated {
				for _, vid := range f.VersionsIDs {
					var v TranslationVersion
					if found, err := unmarshal(vb, vid, &v); err != nil {
						return err
					} else if !found {
						continue
					}
					if filter == fTranslationContains && !strings.Contains(v.Text, filterArg[0]) {
						continue
					}
					f.Versions = append(f.Versions, v)
				}
			}

			if filter == fTranslationContains && len(f.Versions) == 0 {
				continue
			}

			book.Fragments = append(book.Fragments, f)
		}

		return nil
	}); err != nil {
		return Book{}, err
	}
	return book, nil
}

func (db *DB) BookByID(bid uint64) (Book, error) {
	var book Book
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return Book{}, err
	}
	return book, nil
}

func (db *DB) AddBook(title string, fragments []string) (uint64, error) {
	now := time.Now()
	var bid uint64
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		bid, _ = b.NextSequence()

		fb, err := tx.Bucket([]byte("fragments")).CreateBucket(encode(bid))
		if err != nil {
			return err
		}
		_, err = tx.Bucket([]byte("versions")).CreateBucket(encode(bid))
		if err != nil {
			return err
		}

		ids := make([]uint64, len(fragments))
		for i := range fragments {
			fid, _ := fb.NextSequence()
			ids[i] = fid
			if err := marshal(fb, fid, Fragment{
				ID:          fid,
				Created:     now,
				Updated:     now,
				Text:        fragments[i],
				VersionsIDs: []uint64{},
			}); err != nil {
				return err
			}
		}

		if err := marshal(b, bid, Book{
			ID:                  bid,
			Title:               title,
			Created:             now,
			FragmentsTotal:      len(fragments),
			FragmentsTranslated: 0,
			FragmentsIDs:        ids,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return bid, nil
}

func (db *DB) UpdateBookTitle(bid uint64, title string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		book.Title = title
		if err := marshal(b, bid, &book); err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) RemoveBook(bid uint64) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		key := encode(bid)
		if b.Get(key) == nil {
			return ErrNotFound
		}
		return b.Delete(key)
	})
}

func idx(a []uint64, v uint64) int {
	for i, w := range a {
		if w == v {
			return i
		}
	}
	return -1
}
func has(a []uint64, v uint64) bool {
	return idx(a, v) != -1
}

func (db *DB) AddFragment(bid, fidAfter uint64, text string) (Fragment, error) {
	now := time.Now()
	var f Fragment
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		fid, _ := fb.NextSequence()
		if fidAfter == 0 {
			book.FragmentsIDs = append([]uint64{fid}, book.FragmentsIDs...)
		} else {
			findex := idx(book.FragmentsIDs, fidAfter)
			if findex == -1 {
				return ErrNotFound
			}
			fids := make([]uint64, 0, len(book.FragmentsIDs)+1)
			fids = append(fids, book.FragmentsIDs[:findex+1]...)
			fids = append(fids, fid)
			fids = append(fids, book.FragmentsIDs[findex+1:]...)
			book.FragmentsIDs = fids
		}
		book.FragmentsTotal++

		f = Fragment{
			ID:          fid,
			Created:     now,
			Updated:     now,
			Text:        text,
			VersionsIDs: []uint64{},
		}
		if err := marshal(fb, fid, f); err != nil {
			return err
		}

		book.LastActivity = now
		if err := marshal(b, bid, &book); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return Fragment{}, err
	}
	return f, nil
}

func (db *DB) UpdateFragment(bid, fid uint64, text string) error {
	now := time.Now()
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		if !has(book.FragmentsIDs, fid) {
			return ErrNotFound
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		var f Fragment
		if found, err := unmarshal(fb, fid, &f); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		f.Text = text
		if err := marshal(fb, fid, &f); err != nil {
			return err
		}

		book.LastActivity = now
		if err := marshal(b, bid, &book); err != nil {
			return err
		}

		return nil
	})
}

func (db *DB) RemoveFragment(bid, fid uint64) (int, error) {
	now := time.Now()
	var fragmentsTranslated int
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		findex := idx(book.FragmentsIDs, fid)
		if findex == -1 {
			return ErrNotFound
		}
		book.FragmentsIDs = append(book.FragmentsIDs[:findex], book.FragmentsIDs[findex+1:]...)

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		var f Fragment
		if found, err := unmarshal(fb, fid, &f); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		if err := fb.Delete(encode(fid)); err != nil {
			return err
		}

		book.LastActivity = now
		book.FragmentsTotal--
		if len(f.VersionsIDs) > 0 {
			book.FragmentsTranslated--
		}
		fragmentsTranslated = book.FragmentsTranslated

		if err := marshal(b, bid, &book); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return 0, err
	}
	return fragmentsTranslated, nil
}

func (db *DB) StarFragment(bid, fid uint64) error {
	now := time.Now()
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		if !has(book.FragmentsIDs, fid) {
			return ErrNotFound
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		var f Fragment
		if found, err := unmarshal(fb, fid, &f); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		} else if f.Starred {
			return nil
		}

		bb := tx.Bucket([]byte("starred"))
		sfid := make([]byte, 8*2)
		binary.LittleEndian.PutUint64(sfid, bid)
		binary.LittleEndian.PutUint64(sfid[8:], fid)
		if data, err := json.Marshal(StarredFragment{
			Created:    now,
			BookID:     bid,
			BookTitle:  book.Title,
			FragmentID: fid,
			Fragment:   f.Text,
		}); err != nil {
			return err
		} else if err := bb.Put(sfid, data); err != nil {
			return err
		}

		f.Starred = true

		if err := marshal(fb, fid, &f); err != nil {
			return err
		}

		return nil
	})
}

func (db *DB) UnstarFragment(bid, fid uint64) error {
	return db.Update(func(tx *bolt.Tx) error {
		bb := tx.Bucket([]byte("starred"))
		var sf StarredFragment
		sfid := make([]byte, 8*2)
		binary.LittleEndian.PutUint64(sfid, bid)
		binary.LittleEndian.PutUint64(sfid[8:], fid)
		if data := bb.Get(sfid); data == nil {
			return nil
		} else if err := json.Unmarshal(data, &sf); err != nil {
			return err
		} else if sf.BookID != bid || sf.FragmentID != fid {
			return ErrInconsistent
		}
		if err := bb.Delete(sfid); err != nil {
			return err
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		var f Fragment
		if found, err := unmarshal(fb, fid, &f); err != nil {
			return err
		} else if !found {
			return nil
		} else if !f.Starred {
			return nil
		}

		f.Starred = false

		if err := marshal(fb, fid, &f); err != nil {
			return err
		}

		return nil
	})
}

type starredByCreated []StarredFragment

func (t starredByCreated) Len() int { return len(t) }
func (t starredByCreated) Less(i, j int) bool {
	return t[j].Created.Before(t[i].Created)
}
func (t starredByCreated) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (db *DB) Starred() ([]StarredFragment, error) {
	var starred []StarredFragment
	err := db.View(func(tx *bolt.Tx) error {
		starred = starred[:0]
		b := tx.Bucket([]byte("starred"))
		c := b.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var bm StarredFragment
			if err := json.Unmarshal(v, &bm); err != nil {
				return err
			}
			starred = append(starred, bm)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(starredByCreated(starred))
	return starred, nil
}

func (db *DB) CommentFragment(bid, fid uint64, text string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		if !has(book.FragmentsIDs, fid) {
			return ErrNotFound
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		var f Fragment
		if found, err := unmarshal(fb, fid, &f); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}

		f.Comment = text

		if err := marshal(fb, fid, &f); err != nil {
			return err
		}

		return nil
	})
}

func (db *DB) Translate(bid, fid, vidOrZero uint64, text string) (TranslationVersion, int, error) {
	var vers TranslationVersion
	now := time.Now()
	var fragmentsTranslated int
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		if !has(book.FragmentsIDs, fid) {
			return ErrNotFound
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		var f Fragment
		if found, err := unmarshal(fb, fid, &f); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		if vidOrZero != 0 && !has(f.VersionsIDs, vidOrZero) {
			return ErrNotFound
		}

		book.LastActivity = now
		if len(f.VersionsIDs) == 0 {
			book.FragmentsTranslated++
		}
		fragmentsTranslated = book.FragmentsTranslated

		if err := marshal(b, bid, &book); err != nil {
			return err
		}

		vb := tx.Bucket([]byte("versions")).Bucket(encode(bid))
		if vid := vidOrZero; vid == 0 {
			vid, _ = vb.NextSequence()
			f.VersionsIDs = append(f.VersionsIDs, vid)
			if err := marshal(fb, fid, &f); err != nil {
				return err
			}
			vers.ID = vid
			vers.Created = now
		} else {
			if found, err := unmarshal(vb, vid, &vers); err != nil {
				return err
			} else if !found {
				return ErrNotFound
			}
		}

		vers.Updated = now
		vers.Text = text

		if err := marshal(vb, vers.ID, &vers); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return TranslationVersion{}, 0, err
	}
	return vers, fragmentsTranslated, nil
}

func (db *DB) RemoveVersion(bid, fid, vid uint64) (int, error) {
	now := time.Now()
	var fragmentsTranslated int
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		var book Book
		if found, err := unmarshal(b, bid, &book); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		if !has(book.FragmentsIDs, fid) {
			return ErrNotFound
		}

		fb := tx.Bucket([]byte("fragments")).Bucket(encode(bid))
		var f Fragment
		if found, err := unmarshal(fb, fid, &f); err != nil {
			return err
		} else if !found {
			return ErrNotFound
		}
		vindex := idx(f.VersionsIDs, vid)
		if vindex == -1 {
			return ErrNotFound
		}
		f.VersionsIDs = append(f.VersionsIDs[:vindex], f.VersionsIDs[vindex+1:]...)

		if err := marshal(fb, f.ID, &f); err != nil {
			return err
		}

		book.LastActivity = now
		if len(f.VersionsIDs) == 0 {
			book.FragmentsTranslated--
		}
		fragmentsTranslated = book.FragmentsTranslated

		if err := marshal(b, bid, &book); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return 0, err
	}
	return fragmentsTranslated, nil
}
