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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

type App struct {
	db DB
}

var (
	secret = securecookie.GenerateRandomKey(64)
	store  = sessions.NewCookieStore(secret)

	rNewline = regexp.MustCompile(`[\r\n]+`)
)

func logError(err error) {
	log.Println("ERR", err)
}

func internalError(w http.ResponseWriter, err error) {
	logError(err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func u64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func split(s string) []string {
	lines := rNewline.Split(s, -1)
	result := lines[:0]
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		result = append(result, l)
	}
	return result
}

func (a *App) Index(w http.ResponseWriter, r *http.Request) {
	books, err := a.db.BooksByActivity()
	if err != nil {
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if err := indexTmpl.Execute(w, books); err != nil {
		logError(err)
	}
}

func (a *App) Book(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bid, err := u64(vars["book_id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case "GET":
		page := 1
		if p := r.FormValue("page"); p != "" {
			page, err = strconv.Atoi(p)
			if err != nil || page <= 0 {
				http.Error(w, "Invalid page number", http.StatusBadRequest)
				return
			}
		}
		const size = 50
		off := (page - 1) * size

		var book Book
		var err error
		switch r.FormValue("f") {
		case "u":
			book, err = a.db.BookWithTranslations(bid, off, size, fUntranslated)
		case "c":
			book, err = a.db.BookWithTranslations(bid, off, size, fCommented)
		case "s":
			book, err = a.db.BookWithTranslations(bid, off, size, fStarred)
		case "2":
			book, err = a.db.BookWithTranslations(bid, off, size, fWithTwoOrMoreVersions)
		case "o":
			what := r.FormValue("to")
			if what == "" {
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
				return

			}
			book, err = a.db.BookWithTranslations(bid, off, size, fOriginalContains, what)
		case "t":
			what := r.FormValue("tt")
			if what == "" {
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
				return

			}
			book, err = a.db.BookWithTranslations(bid, off, size, fTranslationContains, what)
		case "l":
			book, err = a.db.BookWithTranslations(bid, off, size, fOriginalLength, r.FormValue("comp"), r.FormValue("n"), r.FormValue("unit"))
		default:
			book, err = a.db.BookWithTranslations(bid, off, size, fNone)
		}
		if err != nil {
			if err == ErrNotFound {
				http.NotFound(w, r)
				return
			}
			internalError(w, err)
			return
		}

		pg := Pagination{
			url:          r.URL,
			PageNumber:   page,
			TotalItems:   len(book.FragmentsIDs),
			itemsPerPage: size,
		}

		c, err := r.Cookie("show-orig-toolbox")
		showOrigToolbox := err == nil && c.Value == "1"
		c, err = r.Cookie("fluid")
		fluid := err == nil && c.Value == "1"
		if err := bookTmpl.Execute(w, struct {
			Book
			Pagination
			Query           query
			URL             string
			ShowOrigToolbox bool
			Fluid           bool
		}{
			book,
			pg,
			query{r.URL.Query()},
			r.URL.String(),
			showOrigToolbox,
			fluid,
		}); err != nil {
			logError(err)
		}

	case "POST":
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			http.Error(w, "Title must not be empty!", http.StatusBadRequest)
			return
		}
		err := a.db.UpdateBookTitle(bid, title)
		if err != nil {
			if err == ErrNotFound {
				http.Error(w, "Book not found", 404)
				return
			}
			internalError(w, err)
			return
		}

	case "DELETE":
		err := a.db.RemoveBook(bid)
		if err != nil {
			if err == ErrNotFound {
				http.Error(w, "Book not found", 404)
				return
			}
			internalError(w, err)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (a *App) AddBook(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		sess, _ := store.Get(r, "tl_sess")
		errors := sess.Flashes()
		title, _ := sess.Values["title"].(string)
		delete(sess.Values, "title")
		sess.Save(r, w)
		uploadType := r.FormValue("type")
		if !(uploadType == "plaintext" || uploadType == "csv" || uploadType == "json") {
			uploadType = "plaintext"
		}

		w.Header().Set("Content-Type", "text/html")
		if err := addTmpl.Execute(w, struct {
			Errors []interface{}
			Title  string
			Type   string
		}{
			errors,
			title,
			uploadType,
		}); err != nil {
			logError(err)
		}

	case "POST":
		err := r.ParseMultipartForm(32 * 1024 * 1024)
		if err != nil {
			internalError(w, err)
			return
		}

		title := strings.TrimSpace(r.PostFormValue("title"))
		if title == "" && r.URL.Path != "/add/json" {
			sess, _ := store.Get(r, "tl_sess")
			sess.AddFlash("Title must not be empty!")
			sess.Save(r, w)
			http.Redirect(w, r, r.URL.Path, http.StatusFound)
			return
		}

		var bid uint64
		switch r.URL.Path {
		case "/add":
			content := strings.TrimSpace(r.PostFormValue("content"))
			if content == "" {
				sess, _ := store.Get(r, "tl_sess")
				sess.AddFlash("Content must not be empty!")
				sess.Values["title"] = title
				sess.Save(r, w)
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
				return
			}

			autotranslate := r.PostFormValue("autotranslate") != ""
			bid, err = a.db.AddBook(title, split(content), autotranslate)
			if err != nil {
				internalError(w, err)
				return
			}

		case "/add/csv":
			f, _, err := r.FormFile("csvfile")
			if err != nil {
				internalError(w, err)
				return
			}
			defer f.Close()

			csvr := csv.NewReader(f)
			csvr.FieldsPerRecord = 2
			records, err := csvr.ReadAll()
			if err != nil {
				sess, _ := store.Get(r, "tl_sess")
				sess.AddFlash("Could not parse CSV file.")
				sess.Values["title"] = title
				sess.Save(r, w)
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
				return
			}

			bid, err = a.db.AddTranslatedBook(title, records)
			if err != nil {
				internalError(w, err)
				return
			}

		case "/add/json":
			f, _, err := r.FormFile("jsonfile")
			if err != nil {
				internalError(w, err)
				return
			}
			defer f.Close()

			data, err := ioutil.ReadAll(f)
			if err != nil {
				internalError(w, err)
				return
			}

			bid, err = a.db.ImportBookFromJSON(data)
			if err != nil {
				internalError(w, err)
				return
			}
		}

		http.Redirect(w, r, "/book/"+fmt.Sprint(bid), http.StatusFound)
	}
}

func (a *App) RemoveBook(w http.ResponseWriter, r *http.Request) {
	books, err := a.db.Books()
	if err != nil {
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if err := removeTmpl.Execute(w, books); err != nil {
		logError(err)
	}
}

func (a *App) ReadBook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bid, err := u64(vars["book_id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	book, err := a.db.BookWithTranslations(bid, 0, -1, fNone)
	if err != nil {
		if err == ErrNotFound {
			http.NotFound(w, r)
			return
		}
		internalError(w, err)
		return
	}

	if err := readTmpl.Execute(w, struct {
		Book
		URL          string
		LastVariants bool
	}{
		book,
		r.FormValue("url"),
		r.FormValue("last") != "",
	}); err != nil {
		logError(err)
	}
}

func (a *App) ExportBook(w http.ResponseWriter, r *http.Request) {
	format := r.FormValue("f")
	switch format {
	default:
		http.NotFound(w, r)
		return
	case "plaintext", "plaintext-orig", "csv", "jsonl", "json":
	}

	vars := mux.Vars(r)
	bid, err := u64(vars["book_id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if format == "json" {
		data, err := a.db.ExportBookToJSON(bid)
		if err != nil {
			if err == ErrNotFound {
				http.NotFound(w, r)
			} else {
				internalError(w, err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="book.json"`)
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Write(data)
		return
	}

	book, err := a.db.BookWithTranslations(bid, 0, -1, fNone)
	if err != nil {
		if err == ErrNotFound {
			http.NotFound(w, r)
			return
		}
		internalError(w, err)
		return
	}

	switch format {
	case "plaintext":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="book.txt"`)
		for _, f := range book.Fragments {
			t := f.Text
			if len(f.Versions) > 0 {
				t = f.Versions[0].Text
			}
			fmt.Fprintln(w, t)
			w.Write([]byte{'\n'})
		}
	case "plaintext-orig":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="book.txt"`)
		for _, f := range book.Fragments {
			fmt.Fprintln(w, f.Text)
			w.Write([]byte{'\n'})
		}
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="book.csv"`)
		cw := csv.NewWriter(w)
		for _, f := range book.Fragments {
			t := ""
			if len(f.Versions) > 0 {
				t = f.Versions[0].Text
			}
			cw.Write([]string{f.Text, t})
		}
		cw.Flush()
	case "jsonl":
		w.Header().Set("Content-Type", "application/jsonl; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="book.jsonl"`)
		enc := json.NewEncoder(w)
		for _, f := range book.Fragments {
			t := ""
			if len(f.Versions) > 0 {
				t = f.Versions[0].Text
			}
			enc.Encode(struct {
				Source      string `json:"source"`
				Translation string `json:"translation"`
			}{
				f.Text,
				t,
			})
		}
	}
}

func (a *App) Fragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := u64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}

	book, err := a.db.BookByID(bid)
	if err != nil {
		if err == ErrNotFound {
			http.NotFound(w, r)
			return
		}
		internalError(w, err)
		return
	}

	index := idx(book.FragmentsIDs, fid)
	if index == -1 {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/book/%d?page=%d#f%d", bid, index/50+1, fid), http.StatusFound)
}

func (a *App) AddFragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	after, _ := u64(r.FormValue("after"))
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Error(w, "Text must not be empty!", http.StatusBadRequest)
		return
	}

	f, err := a.db.AddFragment(bid, after, text)
	if err != nil {
		if err == ErrNotFound {
			http.Error(w, "Book or fragment not found", 404)
			return
		}
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Fragment
		SeqNum int           `json:"seq_num"`
		Text   template.HTML `json:"text"`
	}{
		f,
		f.SeqNum,
		render(f.Text),
	})
}

func (a *App) UpdateFragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := u64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Error(w, "Text must not be empty!", http.StatusBadRequest)
		return
	}

	if err := a.db.UpdateFragment(bid, fid, text); err != nil {
		if err == ErrNotFound {
			http.Error(w, "Fragment not found", 404)
			return
		}
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Text template.HTML `json:"text"`
	}{
		render(text),
	})
}

func (a *App) RemoveFragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := u64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}

	fragmentsTranslated, err := a.db.RemoveFragment(bid, fid)
	if err != nil {
		if err == ErrNotFound {
			http.Error(w, "Fragment not found", 404)
			return
		}
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		FragmentsTranslated int `json:"fragments_translated"`
	}{
		fragmentsTranslated,
	})
}

func (a *App) StarFragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := u64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "POST":
		err = a.db.StarFragment(bid, fid)
	case "DELETE":
		err = a.db.UnstarFragment(bid, fid)
	}
	if err != nil {
		internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) CommentFragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := u64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(r.FormValue("text"))

	err = a.db.CommentFragment(bid, fid, text)
	if err != nil {
		if err == ErrNotFound {
			http.Error(w, "Fragment not found", 404)
			return
		}
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Text string `json:"text"`
	}{
		text,
	})
}

func (a *App) Translate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := u64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}
	vid, err := u64(r.FormValue("version_id"))
	if err != nil {
		http.Error(w, "Invalid version ID", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Error(w, "Text must not be empty!", http.StatusBadRequest)
		return
	}

	v, fragmentsTranslated, err := a.db.Translate(bid, fid, vid, text)
	if err != nil {
		if err == ErrNotFound {
			http.Error(w, "Fragment or version not found", 404)
			return
		}
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		TranslationVersion
		ID                  uint64        `json:"id"`
		Text                template.HTML `json:"text"`
		FragmentsTranslated int           `json:"fragments_translated"`
	}{
		v,
		v.ID,
		render(v.Text),
		fragmentsTranslated,
	})
}

func (a *App) RemoveVersion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := u64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := u64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}
	vid, err := u64(vars["version_id"])
	if err != nil {
		http.Error(w, "Invalid version ID", http.StatusBadRequest)
		return
	}

	fragmentsTranslated, err := a.db.RemoveVersion(bid, fid, vid)
	if err != nil {
		if err == ErrNotFound {
			http.Error(w, "Version not found", 404)
			return
		}
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		FragmentsTranslated int `json:"fragments_translated"`
	}{
		fragmentsTranslated,
	})
}

func (a *App) Backup(w http.ResponseWriter, r *http.Request) {
	if err := a.db.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="tl.db"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	}); err != nil {
		logError(err)
	}
}

func (a *App) Scratchpad(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bid, err := u64(vars["book_id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case "GET":
		edit := r.FormValue("edit") != ""
		book, sp, err := a.db.Scratchpad(bid)
		if err != nil {
			if err == ErrNotFound {
				http.NotFound(w, r)
				return
			}
			internalError(w, err)
			return
		}
		if sp.ID == 0 || sp.Text == "" {
			edit = true
		}

		if err := scratchpadTmpl.Execute(w, struct {
			Book Book
			URL  string
			Scratchpad
			Edit bool
		}{
			book,
			r.FormValue("url"),
			sp,
			edit,
		}); err != nil {
			logError(err)
		}

	case "POST":
		text := r.FormValue("text")
		if err := a.db.UpdateScratchpad(bid, text); err != nil {
			internalError(w, err)
			return
		}
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
	}
}
