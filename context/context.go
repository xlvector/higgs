package context

import (
	"bytes"
	"math/rand"
	"strconv"
	"strings"
	"text/template"
	"time"
	"regexp"
	hproxy "github.com/xlvector/higgs/proxy"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/casperjs"
	"github.com/xlvector/higgs/extractor"
	"github.com/xlvector/higgs/jsonpath"
	"github.com/xlvector/higgs/util"
)

func DaysAgo(n int, f string) string {
	return time.Now().AddDate(0, 0, -1*n).Format(f)
}

func NowTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func NowTime(f string) string {
	return time.Now().Format(f)
}

func ChangeTimeFormat(tm, src, dst string) string {
	t, err := time.Parse(src, tm)
	if err != nil {
		return tm
	}
	return t.Format(dst)
}

func AddDate(y, m, d int, f string) string {
	return time.Now().AddDate(y, m, d).Format(f)
}

func FirstDayOfMonthAgo(n int, f string) string {
	tm := time.Now().AddDate(0, -1*n, 0)
	return time.Date(tm.Year(), tm.Month(), 1, 0, 0, 0, 0, time.Local).Format(f)
}

func LastDayOfMonthAgo(n int, f string, afterCurr bool) string {
	tm := time.Now().AddDate(0, -1*(n-1), 0)
	tm = time.Date(tm.Year(), tm.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
	if !afterCurr && time.Now().Sub(tm).Seconds() < 0 {
		return time.Now().Format(f)
	}
	return tm.Format(f)
}

func NowMillTimestamp() string {
	return strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
}

func AESEncodePassword(pwd, key, iv string) string {
	ret, err := util.AESEncodePassword(pwd, key, iv)
	if err != nil {
		dlog.Warn("fail to encode pwd by aes: %v", err)
		return ""
	}
	return ret
}

type Context struct {
	Data		map[string]interface{}
	CJS 		*casperjs.CasperJS
	Proxy		*hproxy.Proxy
	ProxyManager	*hproxy.ProxyManager
}

func NewContext(cjs *casperjs.CasperJS, p *hproxy.Proxy, pm *hproxy.ProxyManager) *Context {
	return &Context{
		Data: 		make(map[string]interface{}),
		CJS:  		cjs,
		Proxy:		p,
		ProxyManager:	pm,
	}
}

func (p *Context) newEmptyTemplate() *template.Template {
	return template.New("").Funcs(template.FuncMap{
		"daysAgo":            DaysAgo,
		"nowTime":            NowTime,
		"addDate":            AddDate,
		"changeTimeFormat":   ChangeTimeFormat,
		"firstDayOfMonthAgo": FirstDayOfMonthAgo,
		"lastDayOfMonthAgo":  LastDayOfMonthAgo,
		"nowTimestamp":       NowTimestamp,
		"nowMillTimestamp":   NowMillTimestamp,
		"randIntn":           rand.Intn,
		"AESEncodePassword":  AESEncodePassword,
		"contains":           strings.Contains,
		"trimPrefix":         strings.TrimPrefix,
		"hasPrefix":          strings.HasPrefix,
		"hasSuffix":          strings.HasSuffix,
		"extractHtml":        p.extractHtml,
		"extractJson":        p.extractJson,
		"extractJsonp":       p.extractJsonp,
		"extractRegex":       p.extractRegex,
		"set":                p.setValue,
		"add":                p.addValue,
		"notEmpty":           p.notEmpty,
		"empty":	      p.empty,
		"readCasper":         p.readCasper,
		"writeCasper":        p.writeCasper,
		"blockTmplProxy":     p.BlockTmplProxy,
		"regexMatch":	      p.RegexMatch,
		"getTimestamp":	      GetTimestamp,
	})
}

func RandRange(min, max int64) int64 {
	if min >= max || min==0 || max==0{
		return max
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Int63n(max-min)+min
}

func GetTimestamp(types, infos string) interface{} {
	dlog.Println(types)
	if types == "spare_time" {
		tomorrow_year,tomorrow_month,tomorrow_date := time.Now().Add(24*time.Hour).Date()
		tomorrow_timestamp := time.Date(tomorrow_year, tomorrow_month, tomorrow_date, 0, 0, 0, 0, time.Local).Unix()
		now_timestamp := time.Now().Unix()
		time_diff := tomorrow_timestamp-now_timestamp
		hours,error := strconv.ParseInt(infos,10,64)
		if error != nil {
			return time_diff
		}
		return RandRange(time_diff,time_diff+hours*3600)
	} else if types == "ranges" {
		nums := strings.Split(infos,"-")
		if len(nums) == 2 {
			minStr := nums[0]
			maxStr := nums[1]
			min,error := strconv.ParseInt(minStr,10,64)
			if error != nil {
				return nil
			}
			max,error := strconv.ParseInt(maxStr,10,64)
			if error != nil {
				return nil
			}
			return RandRange(min,max)
		}
	}
	return nil
}

func (p *Context) RegexMatch(s string, regex string) bool {
	re := regexp.MustCompile(regex)
	result := re.FindAllString(s,-1)
	if len(result) == 0 {
		return false
	} else {
		return true
	}
}

func (p *Context) BlockTmplProxy(tmpl string) bool {
	p.ProxyManager.BlockTmplProxy(tmpl, p.Proxy)
	return true
}

func (p *Context) readCasper() string {
	if p.CJS == nil {
		return ""
	}
	b := p.CJS.ReadChan()
	return string(b)
}

func (p *Context) writeCasper(line string) bool {
	if p.CJS == nil {
		return false
	}
	p.CJS.WriteChan([]byte(line))
	return true
}

func (p *Context) notEmpty(key string) bool {
	if v, ok := p.Data[key]; ok {
		if v == nil {
			return false
		}
		if val, ok2 := v.(string); ok2 {
			if len(val) == 0 {
				return false
			}
		}
		return true
	} else {
		return false
	}
}

func (p *Context) empty(key string) bool {
	if v, ok := p.Data[key]; ok {
		if v == nil {
			return true
		}
		if val, ok2 := v.(string); ok2 {
			if len(val) == 0 {
				return true
			}
		}
		return false
	} else {
		return false
	}
}

func (p *Context) addValue(key string, val int) bool {
	if v, ok := p.Data[key]; ok {
		if vint, ok2 := v.(int); ok2 {
			p.Data[key] = vint + val
		} else {
			return false
		}
	} else {
		p.Data[key] = val
	}
	return true
}

func (p *Context) extractHtml(body, query string) interface{} {
	ret, err := extractor.Extract([]byte(body), query, "html", nil)
	if err != nil {
		dlog.Warn("extract error: %v", err)
	}
	if ret == nil {
		return ""
	}
	return ret
}

func (p *Context) extractJson(body, query string) interface{} {
	ret, err := extractor.Extract([]byte(body), query, "json", nil)
	if err != nil {
		dlog.Warn("extract error: %v", err)
	}
	return ret
}

func (p *Context) extractJsonp(body, query string) interface{} {
	jsonp := jsonpath.FilterJSONP(body)
	ret, err := extractor.Extract([]byte(jsonp), query, "json", nil)
	if err != nil {
		dlog.Warn("extract error of query [%s] body (%s)(%s): %v", query, body, jsonp, err)
	}
	return ret
}

func (p *Context) extractRegex(body, query string) interface{} {
	ret, err := extractor.Extract([]byte(body), query, "regex", nil)
	if err != nil {
		dlog.Warn("extract error: %v", err)
	}
	return ret
}

func (p *Context) Parse(text string) string {
	t, err := p.newEmptyTemplate().Parse(text)
	if err != nil {
		dlog.Warn("parse %s error: %v", text, err)
		return ""
	}
	buf := bytes.NewBufferString("")
	t.Execute(buf, p.Data)
	return buf.String()
}

func (p *Context) Set(k string, v interface{}) {
	p.Data[k] = v
}

func (p *Context) setValue(k string, v interface{}) interface{} {
	p.Set(k, v)
	return v
}

func (p *Context) Get(k string) (interface{}, bool) {
	ret, ok := p.Data[k]
	return ret, ok
}

func (p *Context) Del(k string) {
	delete(p.Data, k)
}

func (p *Context) BatchDel(ks []string) {
	for _, k := range ks {
		p.Del(k)
	}
}
