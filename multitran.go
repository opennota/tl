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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html/charset"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

const multitranBaseURL = "https://www.multitran.ru/c/m.exe?l1=2&l2=1&s="

func (a *App) Multitran(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.FormValue("query"))
	key := "m:" + yoReplacer.Replace(query)
	data, err := cache.Get(key)
	if data == nil {
		url := multitranBaseURL + url.QueryEscape(query)
		resp, err := httpClient.Get(url)
		if err != nil {
			internalError(w, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			internalError(w, httpStatus{resp.StatusCode, url})
			return
		}

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			internalError(w, err)
			return
		}

		cache.Put(key, data)
	}

	utf8r, err := charset.NewReaderLabel("cp1251", bytes.NewReader(data))
	if err != nil {
		internalError(w, err)
		return
	}

	d, err := goquery.NewDocumentFromReader(utf8r)
	if err != nil {
		internalError(w, err)
		return
	}

	sel := d.Find("form#translation")
	var tbl *goquery.Selection
	for i := 0; i < 5; i++ {
		sel = sel.Next()
		if goquery.NodeName(sel) == "table" {
			tbl = sel
			break
		}
	}
	if tbl == nil {
		http.NotFound(w, r)
		return
	}

	d.Find(`span[style]`).Each(func(_ int, sel *goquery.Selection) {
		if sel.AttrOr("style", "") == "color:gray" {
			sel.SetAttr("class", "text-muted")
		}
		sel.RemoveAttr("style")
	})
	d.Find("td[bgcolor] > a:first-child").Each(func(_ int, sel *goquery.Selection) {
		sel.RemoveAttr("href")
		sel.Get(0).Data = "b"
	})

	html, _ := goquery.OuterHtml(sel)

	policy := bluemonday.NewPolicy()
	policy.AllowElements("table", "tbody", "tr", "td", "em", "i", "span", "b")
	policy.AllowAttrs("class").OnElements("span")

	w.Header().Add("Content-Type", "encoding/json")
	json.NewEncoder(w).Encode(struct {
		HTML string `json:"html"`
	}{
		policy.Sanitize(html),
	})
}
