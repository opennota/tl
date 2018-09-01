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
	"regexp"
	"sort"
	"strings"

	"github.com/opennota/morph"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

const (
	synonymsBaseURL = "https://dic.academic.ru/dic.nsf/dic_synonims/"
	synSeekBaseURL  = "https://dic.academic.ru/seek4term.php?json=true&limit=20&did=dic_synonims&q="
)

var (
	rSynonymsURL = regexp.MustCompile(`^https?://dic\.academic\.ru/dic\.nsf/dic_synonims/(\d+)/`)

	yoReplacer = strings.NewReplacer("ё", "е")

	useMorph = morph.Init() == nil
)

func (a *App) Synonyms(w http.ResponseWriter, r *http.Request) {
	type seekResult struct {
		ID    int    `json:"id"`
		Value string `json:"value"`
	}

	query := strings.ToLower(r.FormValue("query"))
	if useMorph && r.FormValue("exact") == "" {
		_, norms, _ := morph.Parse(query)
		if len(norms) > 0 {
			norms = append(norms, yoReplacer.Replace(query))
			sort.Strings(norms)
			words := norms[:0]
			for i, w := range norms {
				w = yoReplacer.Replace(w)
				if i == 0 || words[len(words)-1] != w {
					words = append(words, w)
				}
			}

			if len(words) == 1 {
				query = yoReplacer.Replace(words[0])
			} else {
				w.Header().Add("Content-Type", "encoding/json")
				pre, post := `<a data-id="" morph="1">`, `</a>`
				json.NewEncoder(w).Encode(struct {
					ID      int          `json:"id"`
					Value   string       `json:"value"`
					HTML    string       `json:"html"`
					SeeAlso []seekResult `json:"see_also"`
				}{
					-1,
					query,
					`<div>Select one of: ` + pre + strings.Join(words, post+", "+pre) + post + "</div>",
					[]seekResult{},
				})
				return
			}
		}
	}

	resp, err := httpClient.Get(synSeekBaseURL + url.PathEscape(query))
	if err != nil {
		internalError(w, err)
		return
	}
	defer resp.Body.Close()

	var results []seekResult
	if err := json.NewDecoder(resp.Body).Decode(&struct {
		Results *[]seekResult
	}{
		&results,
	}); err != nil {
		internalError(w, err)
		return
	}
	if len(results) == 0 {
		http.NotFound(w, r)
		return
	}

	resp, err = httpClient.Get(synonymsBaseURL + fmt.Sprint(results[0].ID))
	if err != nil {
		internalError(w, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		internalError(w, fmt.Errorf("HTTP %d", resp.StatusCode))
		return
	}

	d, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		internalError(w, err)
		return
	}

	policy := bluemonday.NewPolicy()
	policy.AllowElements("div", "em", "span", "strong", "u")
	policy.AllowAttrs("data-id").OnElements("a")
	policy.AllowAttrs("class").OnElements("div", "span")

	var entries []string
	d.Find(`div[itemtype$="/term-def.xml"]`).Each(func(_ int, sel *goquery.Selection) {
		sel.Find("[href]").Each(func(_ int, sel *goquery.Selection) {
			href, _ := sel.Attr("href")
			m := rSynonymsURL.FindStringSubmatch(href)
			if m != nil {
				sel.SetAttr("data-id", m[1])
			}
		})
		sel.Find("[style]").Each(func(_ int, sel *goquery.Selection) {
			style, _ := sel.Attr("style")
			switch style {
			case "color: darkgray;", "color: tomato;":
				sel.SetAttr("class", "text-muted")
			case "margin-left:5px", "color: saddlebrown;":
			}
		})
		html, _ := sel.Find("dd").First().Html()
		entries = append(entries, policy.Sanitize(html))
	})

	w.Header().Add("Content-Type", "encoding/json")
	json.NewEncoder(w).Encode(struct {
		ID      int          `json:"id"`
		Value   string       `json:"value"`
		HTML    string       `json:"html"`
		SeeAlso []seekResult `json:"see_also"`
	}{
		results[0].ID,
		results[0].Value,
		strings.Join(entries, ""),
		results[1:],
	})
}
