/*
data = {
    "hello": "word",
    "a": [
        {
            "b": 1
        },
        {
            "b": 2
        }
    ]
}
jsonpath.Query(data, "hello.a[1].b") == 2
*/
package jsonpath

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var NotArrayErr = errors.New("data is not array")

func isCondition(r string) bool {
	return len(r) > 2 && r[0] == '(' && r[len(r)-1] == ')'
}

func isRange(r string) bool {
	return len(r) > 2 && r[0] == '[' && r[len(r)-1] == ']'
}

func parseCondition(buf string) map[string]string {
	tks := strings.Split(strings.Trim(buf, "()"), "&")
	ret := make(map[string]string)
	for _, tk := range tks {
		kv := strings.Split(tk, "=")
		if len(kv) == 2 {
			ret[kv[0]] = kv[1]
		}
	}
	return ret
}

func matchCondition(data map[string]interface{}, c map[string]string) bool {
	for k, v := range c {
		v2, ok := data[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", v2) != v {
			return false
		}
	}
	return true
}

func queryCondition(data interface{}, cstr string) (interface{}, error) {
	c := parseCondition(cstr)
	if vals, ok := data.([]interface{}); ok {
		ret := []interface{}{}
		for _, val := range vals {
			m, ok2 := val.(map[string]interface{})
			if !ok2 {
				continue
			}
			if matchCondition(m, c) {
				ret = append(ret, val)
			}
		}
		return ret, nil
	}
	return nil, NotArrayErr
}

func queryIndex(data interface{}, i int, qs []string) (interface{}, error) {
	if vals, ok := data.([]interface{}); ok {
		i = (i + len(vals)) % len(vals)
		if i < 0 || i >= len(vals) {
			errors.New(fmt.Sprintf("index of of range, size %d, index %d", len(vals), i))
		}
		return query(vals[i], qs)
	}
	return nil, NotArrayErr
}

func queryRange(data interface{}, a, b int, qs []string) (interface{}, error) {
	if vals, ok := data.([]interface{}); ok {
		a = (a + len(vals)) % len(vals)
		b = (b + len(vals)) % len(vals)
		var array []interface{}
		if b == 0 {
			array = vals[a:]
		} else {
			if b >= a {
				array = vals[a:b]
			} else {
				return nil, errors.New(fmt.Sprintf("range b[%d] < a[%d]", b, a))
			}
		}
		var ret []interface{}
		for _, e := range array {
			next, err := query(e, qs)
			if err != nil {
				return nil, err
			}
			ret = append(ret, next)
		}
		return ret, nil
	}
	return nil, NotArrayErr
}

func queryRangeStr(data interface{}, r string, qs []string) (interface{}, error) {
	rg := strings.Trim(r, "[]")
	ab := strings.Split(rg, ":")
	if len(ab) == 1 {
		i, err := strconv.Atoi(ab[0])
		if err != nil {
			return nil, err
		}
		return queryIndex(data, i, qs)
	} else if len(ab) == 2 {
		if len(ab[0]) == 0 {
			ab[0] = "0"
		}
		a, err := strconv.Atoi(ab[0])
		if err != nil {
			return nil, err
		}
		if len(ab[1]) == 0 {
			ab[1] = "0"
		}
		b, err := strconv.Atoi(ab[1])
		if err != nil {
			return nil, err
		}
		return queryRange(data, a, b, qs)
	}
	return nil, errors.New(fmt.Sprintf("range str error: %s", r))
}

func query(data interface{}, qs []string) (interface{}, error) {
	if data == nil || qs == nil || len(qs) == 0 {
		return data, nil
	}

	q := qs[0]
	if isRange(q) {
		return queryRangeStr(data, q, qs[1:])
	} else if isCondition(q) {
		next, err := queryCondition(data, q)
		if err != nil {
			return nil, err
		}
		return query(next, qs[1:])
	} else {
		if m, ok := data.(map[string]interface{}); ok {
			next, ok2 := m[q]
			if !ok2 {
				return nil, nil
			}
			return query(next, qs[1:])
		}

		if vals, ok := data.([]interface{}); ok {
			nexts := []interface{}{}
			for _, val := range vals {
				next, err := query(val, qs)
				if err != nil {
					continue
				}
				nexts = append(nexts, next)
			}
			return nexts, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("invalid data: %v, %v", data, qs))
}

type Json struct {
	data interface{}
}

func NewJson(b []byte) (*Json, error) {
	ret := Json{}
	err := json.Unmarshal(b, &ret.data)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func NewJsonByInterface(d interface{}) (*Json, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return NewJson(b)
}

func (p *Json) Query(q string) (interface{}, error) {
	if q == "*" {
		return p.Data(), nil
	}
	tks := strings.Split(q, ".")
	qs := []string{}
	for _, tk := range tks {
		p := strings.Index(tk, "[")
		if p < 0 {
			qs = append(qs, tk)
		} else {
			qs = append(qs, tk[0:p])
			qs = append(qs, tk[p:])
		}
	}
	return query(p.data, qs)
}

func (p *Json) Data() interface{} {
	return p.data
}

func GetString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if v, ok2 := val.(string); ok2 {
			return v
		}
	}
	return ""
}

func GetBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		if v, ok2 := val.(bool); ok2 {
			return v
		}
	}
	return false
}

func FilterJSONP(s string) string {
	if strings.HasPrefix(s, "(function(){") {
		s = strings.TrimPrefix(s, "(function(){")
		s = strings.TrimSuffix(s, ";})();")
	}

	p0 := strings.Index(s, "{")
	p1 := strings.Index(s, "(")
	if p1 < 0 {
		return s
	} else if p1 >= 0 && p1 > p0 {
		return s
	}
	p1 += 1
	p2 := strings.LastIndex(s, ")")
	if p2 <= p1 {
		return s
	}
	return strings.Trim(s[p1:p2], "\"")
}
