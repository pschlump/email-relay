//
// E M A I L - R E L A Y - Test send email from web.
//
// Copyright (C) Philip Schlump, 2013-2015.
// Version: 1.0.2
// Tested on Wed Sep  2 21:28:25 MDT 2015
//

/*

To Use:

http://52.21.71.211/api/send
	?auth_token=
	&to=
	&toname=
	&from=
	&fromname=
	&subject=
	&bodyhtml=
	&bodytext=

*/

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	template "text/template"

	flags "github.com/jessevdk/go-flags"
	em "github.com/pschlump/emailbuilder"
	"github.com/pschlump/filelib"
	tr "github.com/pschlump/godebug"
	"github.com/pschlump/json" //	"encoding/json"
	"github.com/pschlump/jsonp"
	ms "github.com/pschlump/templatestrings"
	"github.com/zerobfd/mailbuilder"
)

const BuildNo = "034"

/*

TODO:
	1. Auth-Notify ever X emails or erros sent

Sample Configuration File
=========================

{
	"HostIP":"",
	"Port":"80",
	"WWWPath":"/home/ubuntu/www/www_defaulit_com",
	"Auth":"Dg9Tp4ecr8Y3H19lQZtGwFX3ug",
	"Cert":"/home/ubuntu/cfg/cert.pem",
	"Key":"/home/ubuntu/cfg/key.pem",
	"MonitorURL": "no",
	"DebugEmailAddr", "pschlump@gmail.com",
	"ApprovedApps": { "content-pusher" : "yes" }
}

*/

type CfgType struct {
	HostIP            string            //
	Port              string            //
	HttpsPort         string            //
	WWWPath           string            //
	TmplPath          string            //
	Auth              string            // if using IPAuth then set this to "per-ip"
	Cert              string            //
	Key               string            //
	LogFile           string            //
	MonitorURL        string            //
	ApprovedApps      map[string]string // Array of approved applications
	IPAuth            map[string]string //
	DebugEmailAddr    string            // if not empty then this runs on the db_send_to_me flag and sends all email to this address.
	FromEmailAddr     string            //
	MapToEmailAddr    []string          //	A set of email addresses that if you have anybody at that address will get maped to "MapDestAddr" - to match a dest use @pschlump.com
	MapDestAddr       string            // An address to send to as a replacement address
	DebugLog          int               // 0: off, 1: lots, 2: everything
	AuthReloadCfg     string            // Key for reaload of cfg fiels and log rotation
	LogSuccessfulSend string            // if 'y' then will log successful sends to log file
}

// The prupose for MapToEmailAddr , MapDestAddr and RemoteLog are to allow for debug testing of email. (RemoteLog not implemented yet)

var db_send_to_me = false

var Cfg CfgType
var GlobalCfg map[string]string

// var Email *em.EM
var TLS_Up = false
var Msgs_Sent = 0
var Errs = 0
var FoLogFile *os.File
var FoLogX *os.File
var startup_timestamp string

// Note: You MUST supply full hard paths for --cfg and --emailCfgFile if you run this program in a chroot jail!

var opts struct {
	EmailCfgFN string `short:"e" long:"emailCfgFile"  description:"Path to email config"         default:"~/.email/email-config.json"`
	CfgFN      string `short:"c" long:"cfg"           description:"email configuration file"     default:"~/.email/email-auth.cfg"`
	Dir        string `short:"d" long:"dir"           description:"not used"                     default:""`
	Port       string `short:"p" long:"port"          description:"not used"                     default:""`
}

func init() {
	t := time.Now()
	startup_timestamp = t.Format(time.RFC3339Nano)
}

// ===============================================================================================================================================
// Read in configuration file and return configuration or error.   See README.md on what can be set in config file and what the defaults are.
func ReadCfg(fn string) (Cfg CfgType, err error) {
	if fn[0:2] == "~/" {
		fn = ms.HomeDir() + "/" + fn[2:]
	}
	fmt.Printf("Cfg File: %s\n", fn)
	var data []byte
	data, err = ioutil.ReadFile(fn)

	if err != nil {
		fmt.Printf("Cfg Error: %s\n", err)
		e := fmt.Sprintf("Error(12423): File (%s) missing or unreadable error: %v\n", fn, err)
		err = errors.New(e)
	} else {
		fmt.Printf("Cfg Data: %s\n", data)
		Cfg = CfgType{
			Port:           "80",                                   //
			HttpsPort:      "443",                                  //
			WWWPath:        ".",                                    //
			TmplPath:       ".",                                    //
			Cert:           "cert.pem",                             //
			Key:            "key.pem",                              //
			LogFile:        "/var/log/email-relay.log",             //
			MonitorURL:     "http://www.2c-why.com/",               // "http://198.58.107.206/",
			DebugEmailAddr: "",                                     // Must be empty for default to work below
			Auth:           "8842f657-d0e0-4faa-8033-0a90100ee678", // If not set then this UUID will be used.
			AuthReloadCfg:  "cea1154f-4f96-4c49-8144-fe8cbebba08f", //
		}
		err = json.Unmarshal(data, &Cfg)
		if err != nil {
			e := fmt.Sprintf("Error(12402): Invalid format - %v\n", err)
			err = errors.New(e)
		}
		if Cfg.DebugEmailAddr != "" {
			db_send_to_me = true
		}
		if Cfg.DebugLog >= 1 {
			fmt.Printf("Cfg: %s\n", tr.SVarI(Cfg))
		}
	}
	if Cfg.DebugLog >= 100 {
		fmt.Printf("Early Exit - just testing of config\n")
		os.Exit(0)
	}
	if DbDumpMsg {
		fmt.Printf("Cfg: %+v\n", Cfg)
		fmt.Fprintf(FoLogX, "Cfg: %+v\n", Cfg)
		Cfg.DebugLog = 3
	}
	return
}

// ===============================================================================================================================================
// Log if Cfg.Debug >= 2
func LogIt() {
	if Cfg.DebugLog >= 2 {
		fmt.Printf("At %s\n", tr.LF(2))
	}
	// FoLogX, err = os.OpenFile("/home/emailrelay/tmp/out.out", os.O_RDWR|os.O_APPEND, 0660) // open log file
	fmt.Fprintf(FoLogX, "At %s\n", tr.LF(2))
}

// Log and print string if Cfg.Debug >= 2
func LogItS(s string) {
	if Cfg.DebugLog >= 2 {
		fmt.Printf("%s At %s\n", s, tr.LF(2))
	}
	fmt.Fprintf(FoLogX, "%s At %s\n", s, tr.LF(2))
}

func LogItSS(s, t string) {
	if Cfg.DebugLog >= 2 {
		fmt.Printf("%s At %s, -->>%s<<--\n", s, tr.LF(2), t)
	}
	fmt.Fprintf(FoLogX, "%s At %s, -->>%s<<--\n", s, tr.LF(2), t)
}

// ===============================================================================================================================================
// If authorization is per-ip address then check that the IP is valid and that the Key for that IP is valid.
func CheckIpAuth(ip string, auth_token string) bool {
	LogIt()
	if need, ok := Cfg.IPAuth[ip]; ok {
		LogIt()
		if need == auth_token {
			LogIt()
			return true
		}
	}
	LogIt()
	return false
}

// ===============================================================================================================================================
// Report the status of the system - also prints out BuildNo so you can verify that the correct version is running.
// Normally accessed via /api/version or /api/status
func handleVersion(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	io.WriteString(res, jsonp.JsonP(fmt.Sprintf(`{"status":"success","msg":"TLS is %v", "version":"1.0.2 BuildNo %s","Msgs":%d, "Errs":%d, "StartTime":%q}`+"\n",
		TLS_Up, BuildNo, Msgs_Sent, Errs, startup_timestamp), res, req))
}

// ===============================================================================================================================================
// Remotely allow for a reload of the configration file.  Not all confuration stuff is changed.  Mostly this will allow turing on/off debuging
// on the fly.   Also this causes a log file rotation.
func handlereloadConfigFile(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	params := req.URL.Query()
	auth_token := params.Get("auth_token")
	LogItS(fmt.Sprintf("ConfigReload auth_token from URL:%s ", auth_token))

	if auth_token != Cfg.AuthReloadCfg {
		io.WriteString(res, `{"status":"error","msg":"not authorized to reload config"}`)
	} else {
		tCfg, err := ReadCfg(opts.CfgFN) // read in config for this program
		if err != nil {
			fmt.Fprintf(FoLogFile, "Error on reload of config file, %s\n", err)
			io.WriteString(res, `{"status":"error","msg":"error on reload, using old config"}`)
		} else {
			fmt.Fprintf(FoLogFile, `{"status":"success","msg":"TLS is %v", "version":"1.0.2 BuildNo %s","Msgs":%d, "Errs":%d}`+"\n", TLS_Up, BuildNo, Msgs_Sent, Errs)
			FoLogFile.Close()
			Cfg = tCfg
			Msgs_Sent, Errs = 0, 0
			err = os.Rename(Cfg.LogFile, Cfg.LogFile+".old")
			if err != nil {
				fmt.Fprintf(FoLogFile, `{"status":"ErrroOnLogFileRotation","error":%q}`, err)
			} else {
				FoLogFile, err = os.OpenFile(Cfg.LogFile, os.O_RDWR|os.O_APPEND, 0660) // open log file
				if err != nil {
					FoLogFile, err = os.Create(Cfg.LogFile)
					if err != nil {
						panic(err)
					}
				}
				FoLogX, err = os.OpenFile("/home/emailrelay/tmp/out.out", os.O_RDWR|os.O_APPEND, 0660) // open log file
				if err != nil {
					FoLogX, err = os.Create("/home/emailrelay/tmp/out.out")
					if err != nil {
						panic(err)
					}
				}
			}
			io.WriteString(res, `{"status":"success"}`)
		}
	}
}

// ===============================================================================================================================================
// Send an email
func handleSend(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	params := req.URL.Query()
	fmt.Println("GET params:", params)

	LogIt()
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	// when the user acccess the web server via a proxy or load balancer.
	// The above IP address will be the IP address of the proxy or load balancer and not the user's machine.

	// let's get the request HTTP header "X-Forwarded-For (XFF)"
	// if the value returned is not null, then this is the real IP address of the user.
	ipF := req.Header.Get("X-FORWARDED-FOR")
	if ipF != "" {
		ip = ipF
	}

	LogItS(fmt.Sprintf("IP Address:%s ", ip))
	auth_token := params.Get("auth_token")
	LogItS(fmt.Sprintf("auth_token from URL:%s Cfg.Auth:%s ", auth_token, Cfg.Auth))
	if auth_token == Cfg.Auth || (Cfg.Auth == "per-ip" && CheckIpAuth(ip, auth_token)) {

		LogItS("AuthToken Good")
		dTo := params.Get("to")
		dToName := params.Get("toname")
		dFrom := params.Get("from")
		dFromName := params.Get("fromname")
		dSubject := params.Get("subject")
		dBodyHtml := params.Get("bodyhtml")
		dBodyText := params.Get("bodytext")
		dApp := params.Get("app")
		dTmpl := params.Get("tmpl")
		dP0 := params.Get("p0")
		dP1 := params.Get("p1")
		dP2 := params.Get("p2")
		dP3 := params.Get("p3")
		dP4 := params.Get("p4")
		dP5 := params.Get("p5")
		dP6 := params.Get("p6")
		dP7 := params.Get("p7")
		dP8 := params.Get("p8")
		dP9 := params.Get("p9")

		LogItS("Got Params")
		if dTmpl != "" {
			LogItS("Template Specified")

			if _, ok := Cfg.ApprovedApps[dApp]; dApp == "" || !ok {
				LogItS("Error: Not an approved application")
				Errs++
				t := time.Now()
				ts := t.Format(time.RFC3339)
				fmt.Fprintf(FoLogFile, "Error: InvalidApp: %s %q %s\n", ip, dApp, ts)
				io.WriteString(res, jsonp.JsonP(`{"status":"error","msg":"missing invalid app error"}`+"\n", res, req))
				return
			}

			TemplateFn := filepath.Clean(Cfg.TmplPath + filepath.Clean("/"+dApp+"/"+dTmpl))

			if !filelib.Exists(TemplateFn) {
				LogItS(fmt.Sprintf("Error: File did not exist for template, TemplateFileName:%s", TemplateFn))
				Errs++
				t := time.Now()
				ts := t.Format(time.RFC3339)
				fmt.Fprintf(FoLogFile, "Error: MissingTemplate: %s %q %s\n", ip, TemplateFn, ts)
				io.WriteString(res, jsonp.JsonP(`{"status":"error","msg":"missing email template error"}`+"\n", res, req))
				if db_send_to_me {
					TemplateFn = "./debug.tmpl"
				} else {
					return
				}
			}
			LogItS("Ok: Run Template Now")

			g_data = make(map[string]interface{})
			oneRow := make(map[string]interface{})
			oneRow["templateFn"] = TemplateFn
			oneRow["app"] = dApp
			oneRow["tmpl"] = dTmpl
			oneRow["to"] = dTo
			for _, vv := range Cfg.MapToEmailAddr {
				if strings.HasSuffix(dTo, vv) {
					LogIt()
					if Cfg.MapDestAddr != "" {
						dTo = Cfg.MapDestAddr
					} else {
						dTo = "pschlump@yahoo.com"
					}
				}
			}
			if db_send_to_me {
				dTo = Cfg.DebugEmailAddr
			}
			oneRow["toname"] = dToName
			oneRow["from"] = dFrom
			oneRow["fromname"] = dFromName
			oneRow["subject"] = dSubject
			oneRow["bodyhtml"] = dBodyHtml
			oneRow["bodytext"] = dBodyText
			oneRow["p0"] = dP0
			oneRow["p1"] = dP1
			oneRow["p2"] = dP2
			oneRow["p3"] = dP3
			oneRow["p4"] = dP4
			oneRow["p5"] = dP5
			oneRow["p6"] = dP6
			oneRow["p7"] = dP7
			oneRow["p8"] = dP8
			oneRow["p9"] = dP9

			if DbDumpMsg {
				fmt.Fprintf(FoLogFile, "oneRow, %s: %+v\n", tr.LF(), oneRow)
				fmt.Fprintf(FoLogX, "oneRow, %s: %+v\n", tr.LF(), oneRow)
			}

			dSubject = RunTemplate(TemplateFn, "subject", oneRow)
			dBodyHtml = RunTemplate(TemplateFn, "body_html", oneRow)
			dBodyText = RunTemplate(TemplateFn, "body_text", oneRow)
			LogItS("Setup Complete")
			LogItSS("dSubject", dSubject)
			LogItSS("dBodyHtml", dBodyHtml)

		} else {
			LogIt()

			if dToName == "" {
				dToName = dTo
			}
			if dFromName == "" {
				dFromName = dFrom
			}
			if dBodyHtml == "" {
				dBodyHtml = dBodyText
			}
			if dSubject == "" {
				dSubject = "No Subject"
			}

			for _, vv := range Cfg.MapToEmailAddr {
				if strings.HasSuffix(dTo, vv) {
					LogIt()
					if Cfg.MapDestAddr != "" {
						dTo = Cfg.MapDestAddr
					} else {
						dTo = "pschlump@yahoo.com"
					}
				}
			}
			if db_send_to_me {
				dTo = Cfg.DebugEmailAddr
			}

			if dBodyText == "" || dTo == "" || dFrom == "" {
				LogIt()
				Errs++
				t := time.Now()
				ts := t.Format(time.RFC3339)
				fmt.Fprintf(FoLogFile, "Error: MissingParam: %s %q %s\n", ip, dTo, ts)
				io.WriteString(res, jsonp.JsonP(`{"status":"error","msg":"missing parameter error"}`+"\n", res, req))
				return
			}
		}

		if Cfg.FromEmailAddr != "" {
			dFrom = Cfg.FromEmailAddr
		}

		var Email *em.EM

		Email = em.NewEmFile(opts.EmailCfgFN, true) // setup email system
		if Email.Err != nil {
			LogItS("Error: email send - failed to configure email sender")
			fmt.Printf("Fatal: Email is not properly configured, failed to load config file (%s). %s\n", opts.EmailCfgFN, Email.Err)
			return
		}

		LogItS(fmt.Sprintf("To: %s From %s Body %s, line:%s", dTo, dFrom, dBodyHtml, tr.LF()))
		err := Email.To(dTo, dToName).From(dFrom, dFromName).Subject(dSubject).TextBody(dBodyText).HtmlBody(dBodyHtml).SendIt()

		if err != nil {
			LogItS("Error: Got error on email send")
			Errs++
			t := time.Now()
			ts := t.Format(time.RFC3339)
			fmt.Fprintf(FoLogFile, "Error: EmailError: %s %q %s\n", ip, err, ts)
			io.WriteString(res, jsonp.JsonP(fmt.Sprintf(`{"status":"error","msg":"email error","err":%q, "message":{ "to":%q, "toname":%q, "from":%q, "fromname":%q, "subject":%q, "bodyhtml":%q, "bodytext":%q, "app":%q, "tmpl":%q, "p0":%q, "p1":%q, "p2":%q, "p3":%q, "p4":%q, "p5":%q, "p6":%q, "p7":%q, "p8":%q, "p9":%q }}`+"\n", err, dTo, dToName, dFrom, dFromName, dSubject, dBodyHtml, dBodyText, dApp, dTmpl, dP0, dP1, dP2, dP3, dP4, dP5, dP6, dP7, dP8, dP9), res, req))
			return
		} else if Cfg.LogSuccessfulSend == "y" {
			fmt.Fprintf(FoLogFile, `{"status":"success","msg":"email error","err":%q, "message":{ "to":%q, "toname":%q, "from":%q, "fromname":%q, "subject":%q, "bodyhtml":%q, "bodytext":%q, "app":%q, "tmpl":%q, "p0":%q, "p1":%q, "p2":%q, "p3":%q, "p4":%q, "p5":%q, "p6":%q, "p7":%q, "p8":%q, "p9":%q }}`+"\n", err, dTo, dToName, dFrom, dFromName, dSubject, dBodyHtml, dBodyText, dApp, dTmpl, dP0, dP1, dP2, dP3, dP4, dP5, dP6, dP7, dP8, dP9)
		}

		// Email = em.NewEmFile(opts.EmailCfgFN, false) // no new setup of email system
		Email.Message = mailbuilder.NewMessage()
		Email.Message.SetBodyEmpty()

	} else {
		LogItS("Error: Not Authorized")
		Errs++
		t := time.Now()
		ts := t.Format(time.RFC3339)
		fmt.Fprintf(FoLogFile, "Error: NotAuthorized: %s %q %s\n", ip, auth_token, ts)
		io.WriteString(res, jsonp.JsonP(`{"status":"error","msg":"Invalid authorization(1)"}`+"\n", res, req))
		return
	}
	Msgs_Sent++
	io.WriteString(res, jsonp.JsonP(`{"status":"success"}`+"\n", res, req))

}

// =======================================================================================================================================================================
// Templating Section
// =======================================================================================================================================================================

// ------------------------------------------------------------------------------------------------------------------
// Globals for Templates (oooh Ick!)
//		{{g "name"}}  Access a global and return its value from an "interface" of string
//		{{set "name=Value"}} Set a value to constant Value
//		{{ bla | set "name"}} Set a value to Value of pipe
// ------------------------------------------------------------------------------------------------------------------
//var global_data	map[string]string
//func global_init () {
//	global_data = make(map[string]string)
//}
var g_data map[string]interface{}

func init() {
	g_data = make(map[string]interface{})
}

func global_g(b string) string {
	// fmt.Printf ( "XYZZY Inside 'g' -[%s]-\n", g_data[b].(string) )
	return g_data[b].(string)
}

func global_set(args ...string) string {
	if len(args) == 1 {
		b := args[0]
		var re = regexp.MustCompile("([a-zA-Z_][a-zA-Z_0-9]*)=(.*)")
		x := re.FindAllStringSubmatch(b, -1)
		if len(x) == 0 {
			name := x[0][1]
			value := ""
			g_data[name] = value
		} else {
			name := x[0][1]
			value := x[0][2]
			g_data[name] = value
		}
	} else if len(args) == 2 {
		name := args[0]
		value := args[1]
		g_data[name] = value
	} else {
		name := args[0]
		value := strings.Join(args[1:], "")
		g_data[name] = value
	}
	return ""
}

// ===================================================================================================================================================
// Run a template and get the results back as a stirng.
// Sample - used below.
//func ExecuteATemplate(tmpl string, data map[string]interface{}) string {
//	t := template.New("line-template")
//	t, err := t.Parse(tmpl)
//	if err != nil {
//		fmt.Printf("Error(): Invalid template: %s\n", err)
//		return tmpl
//	}
//
//	// Create an io.Writer to write to a string
//	var b bytes.Buffer
//	foo := bufio.NewWriter(&b)
//	err = t.ExecuteTemplate(foo, "line-template", data)
//	if err != nil {
//		fmt.Printf("Error(): Invalid template processing: %s\n", err)
//		return tmpl
//	}
//	foo.Flush()
//	s := b.String() // Fetch the data back from the buffer
//	return s
//}

// ===================================================================================================================================================
// Run a template and get the results back as a stirng.
// This is the primary template runner for sending email.
func RunTemplate(TemplateFn string, name_of string, g_data map[string]interface{}) string {

	rtFuncMap := template.FuncMap{
		"Center":      ms.CenterStr,   //
		"PadR":        ms.PadOnRight,  //
		"PadL":        ms.PadOnLeft,   //
		"PicTime":     ms.PicTime,     //
		"FTime":       ms.StrFTime,    //
		"PicFloat":    ms.PicFloat,    //
		"nvl":         ms.Nvl,         //
		"Concat":      ms.Concat,      //
		"title":       strings.Title,  // The name "title" is what the function will be called in the template text.
		"g":           global_g,       //
		"set":         global_set,     //
		"ifDef":       ms.IfDef,       //
		"ifIsDef":     ms.IfIsDef,     //
		"ifIsNotNull": ms.IfIsNotNull, //
	}

	var b bytes.Buffer
	foo := bufio.NewWriter(&b)

	t, err := template.New("simple-tempalte").Funcs(rtFuncMap).ParseFiles(TemplateFn)
	// t, err := template.New("simple-tempalte").ParseFiles(TemplateFn)
	if err != nil {
		fmt.Printf("Error(12004): parsing/reading template, %s\n", err)
		return ""
	}

	err = t.ExecuteTemplate(foo, name_of, g_data)
	if err != nil {
		fmt.Fprintf(foo, "Error(12005): running template=%s, %s\n", name_of, err)
		return ""
	}

	foo.Flush()
	s := b.String() // Fetch the data back from the buffer

	LogIt()
	fmt.Fprintf(FoLogFile, "Template Output is: ----->%s<-----\n", s)

	return s

}

// ===============================================================================================================================================
func main() {

	GlobalCfg = make(map[string]string) // for compatability with old config system - will be replaced at some point

	junk, err := flags.ParseArgs(&opts, os.Args)

	if len(junk) != 1 { // check for extra command line stuff, if so exit
		fmt.Printf("Extra options at end not allowed\n")
		os.Exit(1)
	}

	if err != nil { // check for errors on CLI
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	// xyzzy1000

	//	Email = em.NewEmFile(opts.EmailCfgFN, true) // setup email system
	//	if Email.Err != nil {
	//		fmt.Printf("Fatal: Email is not properly configured, failed to load config file (%s). %s\n", opts.EmailCfgFN, Email.Err)
	//		os.Exit(1)
	//	}

	Cfg, err = ReadCfg(opts.CfgFN) // read in config for this program
	if err != nil {
		fmt.Printf("Fatal: Email is not properly configured, failed to load config file (%s). %s\n", opts.CfgFN, err)
		os.Exit(1)
	}

	LogItS("Before open of log file")
	FoLogFile, err = os.OpenFile(Cfg.LogFile, os.O_RDWR|os.O_APPEND, 0660) // open log file
	if err != nil {
		FoLogFile, err = os.Create(Cfg.LogFile)
		if err != nil {
			panic(err)
		}
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := FoLogFile.Close(); err != nil {
			panic(err)
		}
	}()

	GlobalCfg["JSON_Prefix"] = "" // more compatability stuff with old config system
	GlobalCfg["monitor_url"] = Cfg.MonitorURL
	LogItS("Log files open")

	monitorGoRoutine("content-pusher", "setup:5 minute", 60) // automatic monotering to the monotoring server

	http.HandleFunc("/api/send", handleSend) // config end points
	http.HandleFunc("/api/version", handleVersion)
	http.HandleFunc("/api/status", handleVersion)
	http.HandleFunc("/api/reloadConfigFile", handlereloadConfigFile)
	http.Handle("/", http.FileServer(http.Dir(Cfg.WWWPath)))
	listen443 := fmt.Sprintf("%s:%s", Cfg.HostIP, Cfg.HttpsPort)
	LogItS("Mux Setup, ready to listen")
	if filelib.Exists(Cfg.Cert) && filelib.Exists(Cfg.Key) { // if have certs then set up TLS/https
		LogIt()
		if Cfg.HostIP == "" {
			fmt.Printf("Serving TLS on port %s\n", Cfg.HttpsPort)
		} else {
			fmt.Printf("Serving TLS on host:port %s\n", listen443)
		}
		// go http.ListenAndServeTLS(":443", Cfg.Cert, Cfg.Key, nil)
		go func() {
			// This code instead supports TLS1.0, TLS1.1 and TLS1.2
			// But note that it may cause you compatibility problems
			// (In particular, TLS_FALLBACK_SCSV is not handled)
			config := &tls.Config{MinVersion: tls.VersionTLS10}
			server := &http.Server{Addr: listen443, TLSConfig: config}
			TLS_Up = true
			err := server.ListenAndServeTLS(Cfg.Cert, Cfg.Key)
			if err != nil {
				TLS_Up = false
				log.Fatal(err)
			}
		}()
	}
	LogIt()

	fmt.Printf("\n====================================================\nBuildNo=%s\n====================================================\n", BuildNo)

	listen := fmt.Sprintf("%s:%s", Cfg.HostIP, Cfg.Port) // Listen on http
	if Cfg.HostIP == "" {
		fmt.Printf("Serving on port %s\n", Cfg.Port)
	} else {
		fmt.Printf("Serving on host:port %s\n", listen)
	}
	log.Fatal(http.ListenAndServe(listen, nil))
}

const DbDumpMsg = true

/* vim: set noai ts=4 sw=4: */
