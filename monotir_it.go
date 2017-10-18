//
// E M A I L - R E L A Y - Test send email from web.
//
// Copyright (C) Philip Schlump, 2013-2015.
// Version: 1.0.2
// Tested on Sun Aug 30 08:59:12 MDT 2015
//

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	tr "github.com/pschlump/godebug"
)

func doGet(client *http.Client, url string) string {
	r1, e0 := client.Get(url)
	if e0 != nil {
		fmt.Printf("Error!!!!!!!!!!! %v, %s\n", e0, tr.LF())
		return "Error"
	}
	rv, e1 := ioutil.ReadAll(r1.Body)
	if e1 != nil {
		fmt.Printf("Error!!!!!!!!!!! %v, %s\n", e1, tr.LF())
		return "Error"
	}
	r1.Body.Close()
	if gp, ok := GlobalCfg["JSON_Prefix"]; ok {
		if string(rv[0:6]) == gp {
			rv = rv[len(gp):]
		}
	}

	return string(rv)
}

func sendIAmAlive(name, note string) {
	if s, ok := GlobalCfg["monitor_url"]; ok {
		if s == "no" {
			return
		}
	}
	client := http.Client{nil, nil, nil, 0}
	if u, ok := GlobalCfg["I_Am_Alive_URL"]; ok {
		doGet(&client, u)
	} else {
		doGet(&client, GlobalCfg["monitor_url"]+"/api/ping_i_am_alive?item="+name+"&note=Ok"+note)
	}
}

func monitorGoRoutine(name, init string, n_sec int) {
	// Send periodic I Am Alive Notices -------------------------------------------------------------------
	sendIAmAlive(name, init)
	ticker := time.NewTicker(time.Duration(n_sec) * time.Second) // should be configurable - but delta_t is 2 min so...
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				sendIAmAlive(name, "")
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
