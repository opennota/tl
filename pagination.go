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
	"net/url"
)

type query struct {
	url.Values
}

func (v query) Page(n int) template.URL {
	if n == 1 {
		v.Values.Del("page")
	} else {
		v.Values.Set("page", fmt.Sprint(n))
	}
	return template.URL(v.Encode())
}

type Pagination struct {
	url          *url.URL
	PageNumber   int
	TotalItems   int
	itemsPerPage int
}

func (p Pagination) TotalPages() int {
	return (p.TotalItems + p.itemsPerPage - 1) / p.itemsPerPage
}

func (p Pagination) Render() template.HTML {
	if p.TotalItems <= p.itemsPerPage {
		return ""
	}
	path := html.EscapeString(p.url.Path)
	q := query{p.url.Query()}
	tp := p.TotalPages()
	var buf bytes.Buffer
	buf.WriteString(`<div class="btn-group btn-group-xs pull-right">`)
	buf.WriteString(`<ul class="pagination pagination-xs">`)
	if p.PageNumber > 2 {
		qry := q.Page(1)
		if qry == "" {
			fmt.Fprintf(&buf, `<li><a href="%s">1</a></li>`, path)
		} else {
			fmt.Fprintf(&buf, `<li><a href="%s?%s">1</a></li>`, path, qry)
		}
	}
	if p.PageNumber > 3 {
		buf.WriteString(`<li><span>...</span></li>`)
	}
	if p.PageNumber > 1 {
		fmt.Fprintf(&buf, `<li><a rel="prev" href="%s?%s">%d</a></li>`, path, q.Page(p.PageNumber-1), p.PageNumber-1)
	}
	fmt.Fprintf(&buf, `<li><span>%d</span></li>`, p.PageNumber)
	if p.PageNumber < tp {
		fmt.Fprintf(&buf, `<li><a rel="next" href="%s?%s">%d</a></li>`, path, q.Page(p.PageNumber+1), p.PageNumber+1)
	}
	if p.PageNumber+2 < tp {
		buf.WriteString(`<li><span>...</span></li>`)
	}
	if p.PageNumber+1 < tp {
		fmt.Fprintf(&buf, `<li><a rel="next" href="%s?%s">%d</a></li>`, path, q.Page(tp), tp)
	}
	buf.WriteString(`</ul></div>`)
	return template.HTML(buf.String())
}

func (p Pagination) RenderPrevNextButtons() template.HTML {
	if p.TotalItems <= p.itemsPerPage {
		return ""
	}
	path := html.EscapeString(p.url.Path)
	q := query{p.url.Query()}
	var buf bytes.Buffer
	buf.WriteString(`<div class="btn-group pull-right">`)
	buf.WriteString(`<ul class="pager"><li>`)
	if p.PageNumber < 2 {
		buf.WriteString(`<span>Previous</span>`)
	} else {
		qry := q.Page(p.PageNumber - 1)
		if qry == "" {
			fmt.Fprintf(&buf, `<a rel="prev" href="%s">Previous</a>`, path)
		} else {
			fmt.Fprintf(&buf, `<a rel="prev" href="%s?%s">Previous</a>`, path, qry)
		}
	}
	buf.WriteString(`</li> <li>`)
	if p.PageNumber >= p.TotalPages() {
		buf.WriteString(`<span>Next</span>`)
	} else {
		fmt.Fprintf(&buf, `<a rel="next" href="%s?%s">Next</a>`, path, q.Page(p.PageNumber+1))
	}
	buf.WriteString(`</li></ul></div>`)
	return template.HTML(buf.String())
}
