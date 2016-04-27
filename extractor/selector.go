package extractor

import (
	"regexp"
	"runtime/debug"
	"strings"
	"net/url"
	"github.com/PuerkitoBio/goquery"
	"github.com/xlvector/dlog"
)

// xpath&attr=&regex&replace=&default=
type HtmlSelector struct {
	Xpath   string
	Attr    string
	Regex   string
	Replace string
	Default string
	Prefix  string
	Suffix  string
	Array   string
}

func NewHtmlSelector(buf string) *HtmlSelector {
	tks := strings.Split(buf, "&")
	ret := HtmlSelector{}
	ret.Xpath = tks[0]
	for _, tk := range tks[1:] {
		kv := strings.Split(tk, "=")
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "attr" {
			ret.Attr = kv[1]
		} else if kv[0] == "regex" {
			ret.Regex = kv[1]
		} else if kv[0] == "replace" {
			ret.Replace = kv[1]
		} else if kv[0] == "default" {
			ret.Default = kv[1]
		} else if kv[0] == "prefix" {
			ret.Prefix = kv[1]
		} else if kv[0] == "suffix" {
			ret.Suffix = kv[1]
		} else if kv[0] == "array" {
			ret.Array = kv[1]
		}
	}
	return &ret
}

func (p *HtmlSelector) PostProcess(s *goquery.Selection) string {
	var ret string
	var err error
	var ok bool

	if len(p.Attr) == 0 || p.Attr == "text" {
		ret = s.Text()
	} else if p.Attr == "html" {
		ret, err = s.Html()
		if err != nil {
			return ""
		}
	} else {
		ret, ok = s.Attr(p.Attr)
		if !ok {
			return ""
		}
	}
	ret = strings.TrimSpace(ret)
	if len(p.Regex) > 0 {
		ret = regexExtract(ret, p.Regex)
	}

	if len(p.Replace) > 0 {
		ret = replaceByCondition(ret, p.Replace)
	}

	if len(ret) == 0 && len(p.Default) > 0 {
		ret = p.Default
	}

	if len(p.Prefix) > 0 {
		prefix,_ := url.QueryUnescape(p.Prefix)
		ret = prefix + ret
	}

	if len(p.Suffix) > 0 {
		suffix,_ := url.QueryUnescape(p.Suffix)
		ret += suffix
	}
	return ret
}

func (p *HtmlSelector) Query(doc *goquery.Selection) interface{} {
	defer func() {
		if err := recover(); err != nil {
			dlog.Warn("run error:%v", err)
			dlog.Warn("selector: %s", p.Xpath)
			debug.PrintStack()
		}
	}()

	var s *goquery.Selection
	if p.Xpath == ":this" {
		s = doc
	} else {
		s = doc.Find(p.Xpath)
	}

	if s.Size() == 1 {
		if p.Array == "true" {
			ret := make([]string, 0, 1)
			s.Each(func(i int, sx *goquery.Selection) {
				ret = append(ret, p.PostProcess(sx))
			})
			return ret
		} else {
			return p.PostProcess(s)
		}
	}

	if s.Size() > 1 {
		if p.Array == "false" {
			return p.PostProcess(s)
		} else {
			ret := make([]string, 0, s.Size())
			s.Each(func(i int, sx *goquery.Selection) {
				ret = append(ret, p.PostProcess(sx))
			})
			return ret
		}
	}
	return nil
}

func regexExtract(buf, regex string) string {
	reg := regexp.MustCompile(regex)
	result := reg.FindAllStringSubmatch(buf, 1)
	if result != nil && len(result) > 0 {
		group := result[0]
		if len(group) > 1 {
			return group[1]
		} else {
			return group[0]
		}
	}
	return ""
}

func FindGroup(reg, body string) []string {
	matcher := regexp.MustCompile(reg)
	result := matcher.FindAllStringSubmatch(body, 1)
	if len(result) > 0 {
		group := result[0]
		return group
	}
	return nil
}

func FindGroupByIndex(reg, body string, index int) string {
	group := FindGroup(reg, body)
	if group != nil && len(group) > index {
		return group[index]
	}
	return ""
}

func replaceByCondition(buf, replace string) string {
	tks := strings.Split(replace, ",")
	for _, tk := range tks {
		kv := strings.Split(tk, ":")
		if len(kv) != 2 {
			continue
		}
		if strings.Contains(buf, kv[0]) {
			return kv[1]
		}
	}
	return buf
}
