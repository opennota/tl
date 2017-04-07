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

func Uint64(s string) (uint64, error) {
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
	err = indexTmpl.Execute(w, books)
	if err != nil {
		logError(err)
	}
}

func (a *App) Book(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bid, err := Uint64(vars["book_id"])
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
			book, err = a.db.BookWithTranslations(bid, off, size, fOriginalContains, r.FormValue("to"))
		case "t":
			book, err = a.db.BookWithTranslations(bid, off, size, fTranslationContains, r.FormValue("tt"))
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
			TotalItems:   len(book.Fragments),
			itemsPerPage: size,
		}

		err = bookTmpl.Execute(w, struct {
			Book
			Pagination
			Query query
		}{
			book,
			pg,
			query{r.URL.Query()},
		})
		if err != nil {
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

		w.Header().Set("Content-Type", "text/html")
		err := addTmpl.Execute(w, struct {
			Errors []interface{}
			Title  string
		}{
			errors,
			title,
		})
		if err != nil {
			logError(err)
		}

	case "POST":
		err := r.ParseMultipartForm(32 * 1024 * 1024)
		if err != nil {
			internalError(w, err)
			return
		}

		title := strings.TrimSpace(r.PostFormValue("title"))
		content := strings.TrimSpace(r.PostFormValue("content"))
		if title == "" || content == "" {
			sess, _ := store.Get(r, "tl_sess")
			if title == "" {
				sess.AddFlash("Title must not be empty!")
			}
			if content == "" {
				sess.AddFlash("Content must not be empty!")
			}
			sess.Values["title"] = title
			sess.Save(r, w)
			http.Redirect(w, r, "/add", http.StatusFound)
			return
		}

		bid, err := a.db.AddBook(title, split(content))
		if err != nil {
			internalError(w, err)
			return
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
	err = removeTmpl.Execute(w, books)
	if err != nil {
		logError(err)
	}
}

func (a *App) ReadBook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bid, err := Uint64(vars["book_id"])
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

	err = readTmpl.Execute(w, book)
	if err != nil {
		logError(err)
	}
}

func (a *App) ExportBook(w http.ResponseWriter, r *http.Request) {
	format := r.FormValue("f")
	if !(format == "plaintext" || format == "csv" || format == "jsonl") {
		http.NotFound(w, r)
		return
	}

	vars := mux.Vars(r)
	bid, err := Uint64(vars["book_id"])
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

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := Uint64(vars["fragment_id"])
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

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	after, _ := Uint64(r.FormValue("after"))
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
		Text template.HTML `json:"text"`
	}{
		f,
		render(f.Text),
	})
}

func (a *App) UpdateFragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := Uint64(vars["fragment_id"])
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

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := Uint64(vars["fragment_id"])
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

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := Uint64(vars["fragment_id"])
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

func (a *App) Starred(w http.ResponseWriter, r *http.Request) {
	starred, err := a.db.Starred()
	if err != nil {
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	err = starredTmpl.Execute(w, starred)
	if err != nil {
		logError(err)
	}
}

func (a *App) CommentFragment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := Uint64(vars["fragment_id"])
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

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := Uint64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}
	vid, err := Uint64(r.FormValue("version_id"))
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

	bid, err := Uint64(vars["book_id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}
	fid, err := Uint64(vars["fragment_id"])
	if err != nil {
		http.Error(w, "Invalid fragment ID", http.StatusBadRequest)
		return
	}
	vid, err := Uint64(vars["version_id"])
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

func (a *App) Admin(w http.ResponseWriter, _ *http.Request) {
	if err := adminTmpl.Execute(w, nil); err != nil {
		logError(err)
	}
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
