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
		"pct":         pct,
		"pct6":        pct6,
		"pretty":      pretty,
		"rfc3339":     rfc3339,
		"datetimeStr": datetimeStr,
		"dec":         dec,
		"inc":         inc,
		"render":      render,
		"renderhl":    renderhl,
	}
	indexTmpl      = mustParse("index")
	addTmpl        = mustParse("add")
	bookTmpl       = mustParse("book")
	removeTmpl     = mustParse("remove")
	readTmpl       = mustParse("read")
	adminTmpl      = mustParse("admin")
	scratchpadTmpl = mustParse("scratchpad")

	rBigWords = regexp.MustCompile(`[^\s<>&;]{32,}`)
	r16Chars  = regexp.MustCompile(`.{16}`)
)

func mustParse(name string) *template.Template {
	name = "/template/" + name + ".html"
	f, err := FS(false).Open(name)
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

func pct6(a, b int) string {
	if b == 0 {
		return ""
	}
	return fmt.Sprintf("%.6g", 100*float64(a)/float64(b))
}

func rfc3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

func datetimeStr(t time.Time) string {
	return t.Format("02-Jan-2006 15:04:05")
}

func pretty(t time.Time) string {
	seconds := time.Since(t).Nanoseconds() / 1e9
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
		return datetimeStr(t)
	}
}

func insertSoftBreaks(s string) string {
	return rBigWords.ReplaceAllStringFunc(s, func(s string) string {
		return r16Chars.ReplaceAllStringFunc(s, func(s string) string {
			return s + "<wbr>"
		})
	})
}

var nl2br = strings.NewReplacer("\n", "<br>\n")

func render(s string) template.HTML {
	s = html.EscapeString(s)
	s = nl2br.Replace(s)
	s = insertSoftBreaks(s)
	return template.HTML(s)
}

func renderhl(s, what string) template.HTML {
	r := regexp.MustCompile("(?i)" + regexp.QuoteMeta(what))
	idxs := r.FindAllStringIndex(s, -1)
	i := 0
	var buf bytes.Buffer
	for _, idx := range idxs {
		from, to := idx[0], idx[1]
		buf.WriteString(html.EscapeString(s[i:from]))
		buf.WriteString("<mark>")
		buf.WriteString(html.EscapeString(s[from:to]))
		buf.WriteString("</mark>")
		i = to
	}
	buf.WriteString(html.EscapeString(s[i:]))
	s = nl2br.Replace(buf.String())
	s = insertSoftBreaks(s)
	return template.HTML(s)
}
