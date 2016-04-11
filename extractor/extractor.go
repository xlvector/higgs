package extractor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/jsonpath"
	"strings"
)

type Context interface {
	Parse(string) string
	Set(string, interface{})
}

func guessDocType(body []byte) string {
	buf := strings.TrimSpace(string(body))
	if len(buf) == 0 {
		return ""
	}
	if buf[0] == '{' && buf[len(buf)-1] == '}' {
		return "json"
	}

	if buf[0] == '[' && buf[len(buf)-1] == ']' {
		return "json"
	}

	if strings.Contains(buf, "<html") && strings.Contains(buf, "</html>") {
		return "html"
	}

	if strings.Contains(buf, "<body") && strings.Contains(buf, "</body>") {
		return "html"
	}

	return ""
}

func deepCopy(i interface{}) interface{} {
	b, _ := json.Marshal(i)
	var ret interface{}
	json.Unmarshal(b, &ret)
	return ret
}

func Extract(body []byte, config0 interface{}, docType string, c Context) (interface{}, error) {
	config := deepCopy(config0)
	if len(docType) == 0 {
		docType = guessDocType(body)
		dlog.Info("guess doc type: %s", docType)
	}
	if docType == "html" {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		return htmlExtract(doc.First(), config, c), nil
	} else if docType == "json" {
		j, err := jsonpath.NewJson(body)
		if err != nil {
			return nil, err
		}
		return jsonExtract(j, config, c), nil
	} else if docType == "jsonp" {
		jsonp := jsonpath.FilterJSONP(string(body))
		return Extract([]byte(jsonp), config, "json", c)
	} else if docType == "regex" {
		return textExtract(string(body), config, c), nil
	} else {
		return nil, errors.New("does not support doc type: " + docType)
	}
}

func parse(v string, c Context) string {
	if c == nil {
		return v
	}
	return c.Parse(v)
}

func jsonUnmarshal(buf string) interface{} {
	dlog.Println(buf)
	var v interface{}
	err := json.Unmarshal([]byte(buf), &v)
	if err != nil {
		return nil
	}
	return v
}

func formatValue(v interface{}) string {
	if val, ok := v.(string); ok {
		return val
	} else if val, ok := v.(float64); ok {
		if int64(val*10000) == int64(val)*10000 {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%f", val)
	} else if val, ok := v.(int64); ok {
		return fmt.Sprintf("%d", val)
	} else if val, ok := v.(bool); ok {
		return fmt.Sprintf("%t", val)
	} else {
		return fmt.Sprintf("%v", val)
	}
}

func jsonQuery(j *jsonpath.Json, qp string, c Context) interface{} {
	if qp == "*" {
		return j.Data()
	}
	tks := strings.SplitN(qp, "||", 2)
	q := tks[0]
	qv := parse(q, c)
	var val interface{}
	if strings.HasPrefix(qv, "c:") {
		val = qv[2:]
	} else {
		tmp, _ := j.Query(qv)
		val = strings.TrimSpace(formatValue(tmp))
	}

	if len(tks) == 2 && c != nil {
		p := tks[1]
		c.Set("_v", val)
		if p == "jsonUnmarshal" {
			return jsonUnmarshal(val.(string))
		} else {
			return c.Parse(p)
		}
	}

	return val
}

func jsonExtract(j *jsonpath.Json, config interface{}, c Context) interface{} {
	if v, ok := config.(string); ok {
		ret := jsonQuery(j, v, c)
		return ret
	}

	if m, ok := config.(map[string]interface{}); ok {
		root := jsonpath.GetString(m, "_root")
		rj := j.Data()
		if len(root) > 0 {
			if c != nil && strings.Contains(root, "{{") {
				root = c.Parse(root)
			}
			rj, _ = j.Query(root)
			if rj == nil {
				return nil
			}
		}
		isArray := jsonpath.GetBool(m, "_array")
		delete(m, "_root")
		delete(m, "_array")
		if isArray {
			ret := []interface{}{}
			if arj, ok2 := rj.([]interface{}); ok2 {
				for _, e := range arj {
					ej, _ := jsonpath.NewJsonByInterface(e)
					ret = append(ret, jsonExtract(ej, config, c))
				}
			}
			return ret
		} else {
			ret := make(map[string]interface{})
			ej, _ := jsonpath.NewJsonByInterface(rj)
			for k, v := range m {
				key := k
				if c != nil && strings.Contains(key, "{{") {
					key = c.Parse(key)
				}
				ret[key] = jsonExtract(ej, v, c)
			}
			return ret
		}
	}
	return nil
}

func regexQuery(text string, qp string, c Context) interface{} {
	tks := strings.SplitN(qp, "||", 2)
	q := tks[0]
	qv := parse(q, c)
	var val interface{}
	if strings.HasPrefix(qv, "c:") {
		val = qv[2:]
	} else {
		val = regexExtract(text, qv)
	}

	if len(tks) == 2 && c != nil {
		p := tks[1]
		c.Set("_v", val)
		return c.Parse(p)
	}

	return val
}

func textExtract(text string, config interface{}, c Context) interface{} {
	if v, ok := config.(string); ok {
		return regexQuery(text, v, c)
	}
	if m, ok := config.(map[string]interface{}); ok {
		ret := make(map[string]interface{})
		for k, v := range m {
			key := k
			if c != nil && strings.Contains(key, "{{") {
				key = c.Parse(key)
			}
			ret[key] = textExtract(text, v, c)
		}
		return ret
	}
	return nil
}

func htmlQuery(s *goquery.Selection, qp string, c Context) interface{} {
	tks := strings.SplitN(qp, "||", 2)
	q := tks[0]
	qv := parse(q, c)
	var val interface{}
	if strings.HasPrefix(qv, "c:") {
		val = qv[2:]
	} else {
		hs := NewHtmlSelector(qv)
		val = hs.Query(s)
	}

	if len(tks) == 2 && c != nil {
		p := tks[1]
		c.Set("_v", val)
		return c.Parse(p)
	}

	return val
}

func htmlExtract(s *goquery.Selection, config interface{}, c Context) interface{} {
	if v, ok := config.(string); ok {
		return htmlQuery(s, v, c)
	}

	if m, ok := config.(map[string]interface{}); ok {
		root := jsonpath.GetString(m, "_root")
		rs := s
		if len(root) > 0 {
			rs = s.Find(root)
			if rs.Size() == 0 {
				return nil
			}
		}
		isArray := jsonpath.GetBool(m, "_array")
		delete(m, "_root")
		delete(m, "_array")
		if isArray {
			ret := []interface{}{}
			rs.Each(func(i int, subSel *goquery.Selection) {
				sub := htmlExtract(subSel, config, c)
				ret = append(ret, sub)
			})
			return ret
		} else {
			ret := make(map[string]interface{})
			for k, v := range m {
				key := k
				if c != nil && strings.Contains(key, "{{") {
					key = c.Parse(key)
				}
				val := htmlExtract(rs, v, c)
				if strings.HasPrefix(k, "@key ") {
					khs := NewHtmlSelector(k[5:])
					keyInter := khs.Query(rs)
					if keyInter == nil {
						continue
					}
					if key, _ = keyInter.(string); ok && len(key) > 0 {
						ret[key] = val
					}
					continue
				}

				if val != nil && strings.HasPrefix(k, "@dup") {
					key = k[6:]
					if tmp, ok := ret[key]; !ok || tmp == nil {
						ret[key] = val
					}
					continue
				}

				ret[key] = val
			}
			return ret
		}
	}

	return nil
}
