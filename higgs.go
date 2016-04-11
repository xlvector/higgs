package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/cmd"
	"github.com/xlvector/higgs/config"
	hproxy "github.com/xlvector/higgs/proxy"
	"github.com/xlvector/higgs/task"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
)

const (
	IpMangerKey = "IP_MANAGER_KEY"
)

var health bool
var taskManager *task.TaskManager

func init() {
	health = true
	taskManager = task.NewTaskManager("./etc/tmpls/")
}

func HandleHealth(w http.ResponseWriter, req *http.Request) {
	if health {
		fmt.Fprint(w, "yes")
	} else {
		http.Error(w, "no", http.StatusNotFound)
	}
}

func HandleStart(w http.ResponseWriter, req *http.Request) {
	health = true
	fmt.Fprint(w, "ok")
}

func HandleShutdown(w http.ResponseWriter, req *http.Request) {
	health = false
	fmt.Fprint(w, "ok")
}

func GetConfig(w http.ResponseWriter, req *http.Request) {
	ret := map[string]interface{}{}
	req.ParseForm()
	fileName := req.FormValue("file")
	tmpl := req.FormValue("tmpl")
	var task *task.Task
	if len(fileName) > 0 {
		task = taskManager.GetByName(fileName)
	} else if len(tmpl) > 0 {
		task = taskManager.Get(tmpl)
	}
	ret["task"] = task
	ret["stat"] = (task != nil)
	result, _ := json.Marshal(ret)
	fmt.Fprintf(w, "%s", result)
}

type CookieEntry struct {
	Name       string
	Value      string
	Domain     string
	Path       string
	Secure     bool
	HttpOnly   bool
	Persistent bool
	HostOnly   bool
}

func FormatCookie(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	cookieJson := req.FormValue("cookie")
	dlog.Info("format %s", cookieJson)
	tmpl := req.FormValue("tmpl")
	cookies := make([]CookieEntry, 0)
	err := json.Unmarshal([]byte(cookieJson), &cookies)
	if err != nil {
		return
	}
	cookieTmplates := config.GetCookieTemplate(tmpl)
	if cookieTmplates == nil {
		return
	}
	ret := make(map[string]map[string]CookieEntry, 0)
	for _, v := range cookies {
		cookieTmplate := cookieTmplates[v.Name]
		if cookieTmplate == nil {
			dlog.Warn("tmpl:%s cookie:%s Not Found", tmpl, v.Name)
			cookieTmplate = cookieTmplates["_DEFAULT"]
		}
		if len(v.Path) > 0 && len(v.Domain) > 0 {
			addToSite(ret, cookieTmplate.Site, v)
			continue
		}
		if len(v.Path) == 0 {
			v.Path = cookieTmplate.Path
		}
		if len(v.Domain) == 0 {
			v.Domain = cookieTmplate.Domain
		}
		v.Secure = cookieTmplate.Secure
		v.HttpOnly = cookieTmplate.HttpOnly
		v.Persistent = cookieTmplate.Persistent
		v.HostOnly = cookieTmplate.HostOnly
		addToSite(ret, cookieTmplate.Site, v)
	}
	result, _ := json.Marshal(ret)
	dlog.Info("formated %s", result)
	fmt.Fprintf(w, "%s", result)
}

func addToSite(ret map[string]map[string]CookieEntry, site string, entry CookieEntry) {
	siteMap := ret[site]
	if siteMap == nil {
		siteMap = make(map[string]CookieEntry, 0)
	}
	siteMap[entry.Domain+";"+entry.Path+";"+entry.Name] = entry
	ret[site] = siteMap
}

func main() {
	runtime.GOMAXPROCS(4)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	port := flag.String("port", "8001", "port number")
	env := flag.String("env", "prod", "env")
	flag.Parse()
	config.Init("./etc/config_" + *env + ".json")
	pm := hproxy.NewProxyManager("./etc/proxy.json")
	service := cmd.NewCasperServer(task.NewTaskCmdFactory(taskManager, pm))

	http.Handle("/submit", service)
	http.HandleFunc("/start", HandleStart)
	http.HandleFunc("/shutdown", HandleShutdown)
	http.HandleFunc("/health", HandleHealth)
	http.HandleFunc("/get/config", GetConfig)
	http.Handle("/proxy", pm)
	http.HandleFunc("/format_cookie", FormatCookie)
	http.Handle("/site/",
		http.StripPrefix("/site/",
			http.FileServer(http.Dir("./site"))))

	l, e := net.Listen("tcp", ":"+*port)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
}
