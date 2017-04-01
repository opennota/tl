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

//go:generate esc -o static.go css js fonts template

package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

var (
	addr       = flag.String("http", "", "HTTP service address (default :$PORT or :3000)")
	dataSource = flag.String("db", "tl.db", "Path to the translation database")
)

func main() {
	flag.Parse()

	if *addr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "3000"
		}
		*addr = "127.0.0.1:" + port
	}

	db, err := OpenDatabase(*dataSource, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	app := App{db}

	r := mux.NewRouter()

	r.HandleFunc("/", app.Index).Methods("GET")
	r.HandleFunc("/add", app.AddBook).Methods("GET", "POST")
	r.HandleFunc("/remove", app.RemoveBook).Methods("GET")
	r.HandleFunc("/starred", app.Starred).Methods("GET")
	r.HandleFunc("/admin", app.Admin).Methods("GET")
	r.HandleFunc("/backup", app.Backup).Methods("GET")
	r.HandleFunc(`/book/{book_id:[0-9]+}`, app.Book).
		Methods("GET", "POST", "DELETE")
	r.HandleFunc(`/book/{book_id:[0-9]+}/read`, app.ReadBook).
		Methods("GET")
	r.HandleFunc(`/book/{book_id:[0-9]+}/export`, app.ExportBook).
		Methods("GET")
	r.HandleFunc("/book/{book_id:[0-9]+}/{fragment_id:[0-9]+}", app.Fragment).
		Methods("GET")
	r.HandleFunc("/book/{book_id:[0-9]+}/fragments", app.AddFragment).
		Methods("POST")
	r.HandleFunc("/book/{book_id:[0-9]+}/{fragment_id:[0-9]+}", app.UpdateFragment).
		Methods("POST")
	r.HandleFunc("/book/{book_id:[0-9]+}/{fragment_id:[0-9]+}", app.RemoveFragment).
		Methods("DELETE")
	r.HandleFunc("/book/{book_id:[0-9]+}/{fragment_id:[0-9]+}/star", app.StarFragment).
		Methods("POST", "DELETE")
	r.HandleFunc("/book/{book_id:[0-9]+}/{fragment_id:[0-9]+}/comment", app.CommentFragment).
		Methods("POST")
	r.HandleFunc("/book/{book_id:[0-9]+}/{fragment_id:[0-9]+}/translate", app.Translate).
		Methods("POST")
	r.HandleFunc("/book/{book_id:[0-9]+}/{fragment_id:[0-9]+}/{version_id:[0-9]+}", app.RemoveVersion).
		Methods("DELETE")

	r.Handle("/{_:css|js|fonts}/{.*}", http.FileServer(FS(true))).Methods("GET")

	log.Fatal(http.ListenAndServe(*addr, r))
}
