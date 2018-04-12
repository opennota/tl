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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

const oxfordBaseURL = "https://en.oxforddictionaries.com/definition/"

func (a *App) Definitions(w http.ResponseWriter, r *http.Request) {
	resp, err := c.Get(oxfordBaseURL + url.QueryEscape(r.FormValue("query")))
	if err != nil {
		internalError(w, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			http.NotFound(w, r)
		} else {
			internalError(w, fmt.Errorf("HTTP %d", resp.StatusCode))
		}
		return
	}

	d, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		internalError(w, err)
		return
	}

	policy := bluemonday.NewPolicy()
	policy.AllowElements("button", "div", "em", "h2", "h3", "li", "ol", "p", "section", "span", "strong", "sup", "ul")
	policy.AllowAttrs("class").Globally()

	var result []string
	d.Find(`section.gramb, section.etym, .hwg`).Each(func(_ int, sel *goquery.Selection) {
		sel.Find(".rsbtn_play, .speaker, .ipaLink, .exs + a").
			Each(func(_ int, sel *goquery.Selection) {
				sel.Remove()
			})
		html, _ := goquery.OuterHtml(sel)
		result = append(result, policy.Sanitize(html))
	})

	if len(result) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Add("Content-Type", "encoding/json")
	json.NewEncoder(w).Encode(struct {
		HTML string `json:"html"`
	}{
		strings.Join(result, ""),
	})
}
