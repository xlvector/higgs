package task

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/xlvector/higgs/jsonpath"
	"log"
	"net/http"
	"testing"
	"time"
)

func MockSite() {
	http.HandleFunc("/first", func(rw http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.String())
		query := r.URL.Query().Get("q")
		http.SetCookie(rw, &http.Cookie{Name: "hello", Value: "world"})
		fmt.Fprintf(rw, "<html><body><div id=\"query\">%s</div><div id=\"user-agent\">%s</div><div id=\"referer\">%s</div></body></html>", query, r.Header.Get("User-Agent"), r.Header.Get("Referer"))
	})
	http.ListenAndServe(":20001", nil)
}

func TestStep(t *testing.T) {
	go MockSite()
	time.Sleep(time.Second)
	c := []byte(stepConfig)
	var step Step
	json.Unmarshal([]byte(c), &step)
	d := NewDownloader(nil, nil, "./", nil)
	d.Context.Set("tmpl", "mock")
	d.Context.Set("query", "001")
	err := step.Do(d, nil, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if v, ok := d.Context.Get("q"); ok {
		assert.Equal(t, "001", v)
	}
	if v, ok := d.Context.Get("user-agent"); ok {
		assert.Equal(t, "MockSite", v)
	}
	if v, ok := d.Context.Get("referer"); ok {
		assert.Equal(t, "http://www.baidu.com/", v)
	}
	if v, ok := d.Context.Get("cookie_hello"); ok {
		assert.Equal(t, "world", v)
	}
	j, _ := jsonpath.NewJsonByInterface(d.ExtractorResults)
	if v, err := j.Query("mock.referer"); err == nil {
		assert.Equal(t, "http://www.baidu.com/", fmt.Sprintf("%v", v))
	}

	if v, err := j.Query("mock.const"); err == nil {
		assert.Equal(t, "http://www.baidu.com/", fmt.Sprintf("%v", v))
	}
}

var stepConfig = `
{
    "condition": "{{eq .tmpl \"mock\"}}",
    "page": "http://127.0.0.1:20001/first?q={{.query}}",
    "header": {
        "User-Agent": "MockSite",
        "Referer": "http://www.baidu.com/"
    },
    "doc_type": "html",
    "context_opers": [
        "{{extractHtml ._body \"#query\" | set \"q\"}}",
        "{{extractHtml ._body \"#user-agent\" | set \"user-agent\"}}",
        "{{extractHtml ._body \"#referer\" | set \"referer\"}}"
    ],
    "extractor": {
        "mock": {
            "query": "#query&replace=001:100",
            "user_agent": "#user-agent",
            "referer": "#referer",
            "const": "c:{{.referer}}"
        }
    }
}
`
