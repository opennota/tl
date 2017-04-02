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
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"
)

var (
	funcs = template.FuncMap{
		"pct":      pct,
		"pretty":   pretty,
		"rfc3339":  rfc3339,
		"dec":      dec,
		"inc":      inc,
		"render":   render,
		"renderhl": renderhl,
	}
	indexTmpl   = mustParse("index")
	addTmpl     = mustParse("add")
	bookTmpl    = mustParse("book")
	removeTmpl  = mustParse("remove")
	starredTmpl = mustParse("starred")
	readTmpl    = mustParse("read")
	adminTmpl   = mustParse("admin")

	rBigWords = regexp.MustCompile(`[^\s<>&;]{32,}`)
	r16Chars  = regexp.MustCompile(`.{16}`)
)

func mustParse(name string) *template.Template {
	name = "/template/" + name + ".html"
	f, err := FS(true).Open(name)
	if err != nil {
		log.Fatalf("mustParse: cannot open %q: %v", name, err)
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("mustParse: error while reading from %q: %v", name, err)
	}
	return template.Must(template.New("").Funcs(funcs).Parse(string(data)))
}

func dec(a int) int { return a - 1 }

func inc(a int) int { return a + 1 }

func pct(a, b int) int {
	if b == 0 {
		return 0
	}
	return 100 * a / b
}

func rfc3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

func pretty(t time.Time) string {
	seconds := time.Since(t).Nanoseconds() / 1000000000
	days := seconds / (60 * 60 * 24)
	switch {
	case days < 0:
		return "somewhen in the future"
	case days == 0:
		if seconds < 60*60 {
			minutes := seconds / 60
			switch minutes {
			case 0:
				return "just now"
			case 1:
				return "1 minute ago"
			default:
				return fmt.Sprintf("%d minutes ago", minutes)
			}
		}
		hours := seconds / (60 * 60)
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case days < 7:
		if days == 1 {
			return "Yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	case days < 31:
		weeks := days / 7
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		return t.Format("02.01.2006 15:04:05")
	}
}

func insertSoftBreaks(s string) string {
	return rBigWords.ReplaceAllStringFunc(s, func(s string) string {
		return r16Chars.ReplaceAllStringFunc(s, func(s string) string {
			return s + "<wbr>"
		})
	})
}

func render(s string) template.HTML {
	s = html.EscapeString(s)
	s = strings.Replace(s, "\n", "<br>\n", -1)
	s = insertSoftBreaks(s)
	return template.HTML(s)
}

func renderhl(s, what string) template.HTML {
	ss := strings.Split(s, what)
	whatho := html.EscapeString(what)
	whatho = strings.Replace(whatho, "\n", "<br>\n", -1)
	whatho = "<mark>" + whatho + "</mark>"
	var buf bytes.Buffer
	for i, s := range ss {
		if i != 0 {
			buf.WriteString(whatho)
		}
		s = html.EscapeString(s)
		s = strings.Replace(s, "\n", "<br>\n", -1)
		buf.WriteString(s)
	}
	return template.HTML(insertSoftBreaks(buf.String()))
}
