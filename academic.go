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
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

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

type seekResult struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

func fitKey(key string) string {
	if len(key) <= 127 {
		return key
	}
	cksum := crc32.ChecksumIEEE([]byte(key))
	key = key[:127-10]
	r, size := utf8.DecodeLastRuneInString(key)
	if r == utf8.RuneError {
		key = key[:len(key)-size]
	}
	return key + fmt.Sprintf("..%08x", cksum)
}

func seekSynonym(query string) ([]seekResult, error) {
	key := "a:s:0:" + query
	key = fitKey(key)
	data, _ := cache.Get(key)
	if data == nil {
		url := synSeekBaseURL + url.QueryEscape(query)
		resp, err := httpClient.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, httpStatus{resp.StatusCode, url}
		}

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		cache.Put(key, data)
	}

	rd := bytes.NewReader(data)
	var results []seekResult
	if err := json.NewDecoder(rd).Decode(&struct {
		Results *[]seekResult
	}{
		&results,
	}); err != nil {
		return nil, err
	}

	return results, nil
}

func (a *App) Academic(w http.ResponseWriter, r *http.Request) {
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

	query = yoReplacer.Replace(query)
	results, err := seekSynonym(query)
	key := "a:s:1:" + query
	key = fitKey(key)
	data, _ := cache.Get(key)

	if err != nil {
		if data == nil {
			internalError(w, err)
			return
		}
		logError(err)
	}
	if len(results) == 0 {
		if data == nil {
			http.NotFound(w, r)
			return
		}
		results = []seekResult{{0, query}}
	}

	if data == nil {
		url := synonymsBaseURL + fmt.Sprint(results[0].ID)
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

	d, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
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
