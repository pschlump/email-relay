package jsonp

import (
	"net/http"
	"testing"
)

type TestAAAResponseWriter struct {
	hdr    http.Header
	buf    []byte
	status int
}

func (ho TestAAAResponseWriter) Header() http.Header {
	return http.Header(ho.hdr)
}

func (ho TestAAAResponseWriter) Write(b []byte) (int, error) {
	return 0, nil
}

func (ho TestAAAResponseWriter) WriteHeader(code int) {
	ho.status = code
}

func Test301Redirect(t *testing.T) {

}

func Test_Exists(t *testing.T) {
	/*
		if Exists("./no-such-file.do-not-create") {
			t.Errorf("Expected false, file should not exist\n")
		}
		if !Exists("./test_data") {
			t.Errorf("Expected true, check of directory\n")
		}
		if !Exists("./test_data/file-exists") {
			t.Errorf("Expected true, check of directory\n")
		}
	*/
	SetJsonPrefix(")]}',\n")
	if JSON_Prefix != ")]}',\n" {
		t.Errorf("Expected prefix to be set, not set correctly\n")
	}

	var bob = "{\"bob\":\"bob\"}\n"

	if PrependPrefix(bob) != ")]}',\n{\"bob\":\"bob\"}\n" {
		t.Errorf("Expected prefix to be used, not used correctly\n")
	}

	SetJsonPrefix("")

	res := TestAAAResponseWriter{
		hdr:    make(http.Header),
		buf:    make([]byte, 0, 200),
		status: 0,
	}

	s := JsonP_Param(bob, res, "callback000")
	// fmt.Printf("s >%s<\n", s)
	if s != "callback000({\"bob\":\"bob\"}\n);" {
		t.Errorf("Expected JSONp callback, got >%s< \n", s)
	}
	// TODO: check that headers are set corectly

	// req, _ := http.NewRequest("GET", "http://localhost:8204/api/status?id=xyzzy", nil)		// Test requries a server at this locaiton
	req, _ := http.NewRequest("GET", "http://google.com/", nil)
	_ = req

	// TODO: func JsonP(s string, res http.ResponseWriter, req *http.Request) string {

}
