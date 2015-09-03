//
// Copyright (C) Philip Schlump, 2013-2015.
// Version: 1.0.2
// Tested on Wed Sep  2 21:28:25 MDT 2015
//
package jsonp

import (
	"fmt"
	"net/http"
	"net/url"
)

var JSON_Prefix string = ""

func SetJsonPrefix(p string) {
	JSON_Prefix = p
}

// -------------------------------------------------------------------------------------------------
// Take a string 's' and if a get parameter "callback" is specified then format this for JSONP.
// -------------------------------------------------------------------------------------------------
func JsonP(s string, res http.ResponseWriter, req *http.Request) string {

	u, _ := url.ParseRequestURI(req.RequestURI)
	m, _ := url.ParseQuery(u.RawQuery)
	callback := m.Get("callback")
	if callback != "" {
		res.Header().Set("Content-Type", "application/javascript")
		return fmt.Sprintf("%s(%s);", callback, s)
	} else {
		return JSON_Prefix + s
	}
}
