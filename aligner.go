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
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const rowsPerPage = 50 // keep in sync with aligner.js

var (
	left  [][]string
	right [][]string

	nonceMtx sync.Mutex
	nonce    uint64

	rSpace = regexp.MustCompile(`\s+`)
)

func splitToWords(s string) [][]string {
	var ss [][]string
	for _, s := range rNewline.Split(s, -1) {
		s = strings.TrimSpace(s)
		if s != "" {
			ss = append(ss, rSpace.Split(s, -1))
		}
	}
	return ss
}

func (a *App) Aligner(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		switch what := r.FormValue("download"); what {
		default:
			pageNumber, _ := strconv.Atoi(r.FormValue("page"))
			if pageNumber == 0 {
				pageNumber = 1
			}
			totalRows := max(len(left), len(right))
			offset := (pageNumber - 1) * rowsPerPage
			if totalRows > 0 && offset >= totalRows {
				http.Redirect(w, r, "/aligner", http.StatusSeeOther)
				return
			}
			totalPages := (totalRows + rowsPerPage - 1) / rowsPerPage

			// XXX don't modify these on GET.
			if offset > len(left) {
				left = append(left, make([][]string, offset-len(left))...)
			}
			if offset > len(right) {
				right = append(right, make([][]string, offset-len(right))...)
			}

			nonceMtx.Lock()
			nextNonce := nonce + 1
			nonceMtx.Unlock()

			w.Header().Set("Content-Type", "text/html")
			if err := alignerTmpl.Execute(w, struct {
				Left       [][]string
				Right      [][]string
				PageNumber int
				TotalPages int
				Nonce      uint64
			}{
				left[offset:min(offset+rowsPerPage, len(left))],
				right[offset:min(offset+rowsPerPage, len(right))],
				pageNumber,
				totalPages,
				nextNonce,
			}); err != nil {
				logError(err)
			}

		case "left", "right":
			w.Header().Add("Content-Type", "text/plain; charset=utf-8")
			w.Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, what))
			text := left
			if what == "right" {
				text = right
			}
			for _, ss := range text {
				fmt.Fprintln(w, strings.Join(ss, " "))
			}

		case "csv":
			w.Header().Add("Content-Type", "text/csv; charset=utf-8")
			w.Header().Add("Content-Disposition", `attachment; filename="book.csv"`)
			n := min(len(left), len(right))
			cw := csv.NewWriter(w)
			for i := 0; i < n; i++ {
				if len(left[i]) == 0 && len(right[i]) == 0 {
					continue
				}
				err := cw.Write([]string{
					strings.Join(left[i], " "),
					strings.Join(right[i], " "),
				})
				if err != nil {
					log.Print(err)
				}
			}
			if len(left) < len(right) {
				for i := len(left); i < len(right); i++ {
					err := cw.Write([]string{
						"",
						strings.Join(right[i], " "),
					})
					if err != nil {
						log.Print(err)
					}
				}
			} else if len(right) < len(left) {
				for i := len(right); i < len(left); i++ {
					err := cw.Write([]string{
						strings.Join(left[i], " "),
						"",
					})
					if err != nil {
						log.Print(err)
					}
				}
			}
			cw.Flush()
			if err := cw.Error(); err != nil {
				log.Print(err)
			}
		}

	case "POST":
		nonceMtx.Lock()
		expectedNonce := nonce + 1
		if clientNonce, _ := u64(r.FormValue("nonce")); clientNonce != expectedNonce {
			nonceMtx.Unlock()
			http.Error(w, "", http.StatusConflict)
			return
		}
		nonce++
		nonceMtx.Unlock()

		pageNumber, _ := strconv.Atoi(r.FormValue("page"))
		if pageNumber < 0 {
			pageNumber = 1
		}
		i, _ := strconv.Atoi(r.FormValue("row"))
		j, _ := strconv.Atoi(r.FormValue("word"))
		side := r.FormValue("side")
		offset := (pageNumber - 1) * rowsPerPage
		i += offset

		switch r.FormValue("op") {
		default:
			err := r.ParseMultipartForm(32 * 1024 * 1024)
			if err != nil {
				log.Println(err)
			}
			left = splitToWords(r.PostFormValue("left"))
			right = splitToWords(r.PostFormValue("right"))
			http.Redirect(w, r, "/aligner", http.StatusSeeOther)

		case "split":
			if side == "left" {
				left = append(left, nil)
				copy(left[i+2:], left[i+1:])
				left[i+1] = left[i][j:]
				left[i] = left[i][:j:j]
			} else {
				right = append(right, nil)
				copy(right[i+2:], right[i+1:])
				right[i+1] = right[i][j:]
				right[i] = right[i][:j:j]
			}
			w.WriteHeader(http.StatusNoContent)

		case "join":
			var joined, bottom []string
			if side == "left" {
				if i+1 < len(left) {
					joined = left[i+1]
					left[i] = append(left[i], left[i+1]...)
					left = append(left[:i+1], left[i+2:]...)
				}
			} else {
				if i+1 < len(right) {
					joined = right[i+1]
					right[i] = append(right[i], right[i+1]...)
					right = append(right[:i+1], right[i+2:]...)
				}
			}
			offset += rowsPerPage - 1

			s := left
			if side != "left" {
				s = right
			}
			if offset < len(s) {
				bottom = s[offset]
			}

			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode([][]string{joined, bottom})

		case "rm":
			if i < len(left) {
				left = append(left[:i], left[i+1:]...)
			}
			if i < len(right) {
				right = append(right[:i], right[i+1:]...)
			}
			offset += rowsPerPage - 1

			var s, t []string
			if offset < len(left) {
				s = left[offset]
			}
			if offset < len(right) {
				t = right[offset]
			}

			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode([][]string{s, t})

		case "edit":
			words := rSpace.Split(r.FormValue("text"), -1)
			if side == "left" {
				left[i] = words
			} else {
				right[i] = words
			}

			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode(words)

		case "swap":
			left, right = right, left

		case "clear":
			left = nil
			right = nil

		case "import":
			title := strings.TrimSpace(r.FormValue("title"))
			if title == "" {
				http.Error(w, "Title must not be empty!", http.StatusBadRequest)
				return
			}
			records := make([][]string, 0, len(left))
			for i, words := range left {
				if len(words) == 0 {
					continue
				}
				translation := ""
				if i < len(right) {
					translation = strings.Join(right[i], " ")
				}
				records = append(records, []string{
					strings.Join(words, " "),
					translation,
				})
			}
			bid, err := a.db.AddTranslatedBook(title, records)
			if err != nil {
				internalError(w, err)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode(bid)
		}

	default:
		http.Error(w, "method now allowed", http.StatusMethodNotAllowed)
	}
}
