package extractor

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/xlvector/higgs/jsonpath"
	"strings"
	"testing"
)

func TestGoQuery(t *testing.T) {
	html := `
        <html>
            <body>
                <div id="a">
                    <div>001</div>
                    <div>002</div>
                    <div>
                        <div>003</div>
                    </div>
                </div>
                <div>
                    <div>003</div>
                    <div>004</div>
                </div>
            </body>
        </html>
    `

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Error(err)
		return
	}
	s := doc.First()
	assert.Equal(t, "001", s.Find("#a > div:nth-child(1)").Text())
	assert.Equal(t, "002", s.Find("#a > div:nth-child(2)").Text())
	assert.Equal(t, 2, s.Find("div:containsOwn('003')").Size())
}

func TestHtmlSelector(t *testing.T) {
	html := `
        <html>
            <head><title>fasdfs</title></head>
            <body>
                <div id="a">
                    <div>001</div>
                    <div>002</div>
                    <div id="b">sddssd<a>003abc</a>dss</div>
                </div>
                <div>
                    <div>003</div>
                    <div>004</div>
                </div>
            </body>
        </html>
    `
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	s := doc.First()

	{
		sel := NewHtmlSelector("#a div:contains('002')&replace=2:aaa")
		assert.Equal(t, "aaa", fmt.Sprintf("%v", sel.Query(s)))
	}
	{
		sel := NewHtmlSelector("#a div:contains('a')&regex=([0-9]+)")
		assert.Equal(t, "003", fmt.Sprintf("%v", sel.Query(s)))
	}
	{
		sel := NewHtmlSelector("#b&attr=html")
		assert.Equal(t, "sddssd<a>003abc</a>dss", fmt.Sprintf("%v", sel.Query(s)))
	}
}

func TestJsonExtractor(t *testing.T) {
	data := `
        {
            "query": "aaa",
            "shops": [
                {
                    "name": "001",
                    "price": 0
                },
                {
                    "name": "002",
                    "price": 1
                }
            ]
        }
    `
	j, _ := jsonpath.NewJson([]byte(data))
	var config interface{}
	err := json.Unmarshal([]byte(`
        {
            "query": "query",
            "result": {
                "shop": {
                    "_root": "shops.(name:001)[0]",
                    "name": "name",
                    "price": "price"
                }
            }
        }
    `), &config)
	if err != nil {
		t.Error(err)
		return
	}
	d := jsonExtract(j, config, nil)
	jd, _ := jsonpath.NewJsonByInterface(d)
	t.Log(d)

	if val, err := jd.Query("result.shop.name"); err != nil || val == nil {
		t.Error(err)
	}

	Extract([]byte(data), config, "json", nil)
}

func TestRegexExtractor(t *testing.T) {
	buf := "jQuery172047437679511494935_1455015551115({resultCode:\"0000\",redirectURL:\"http://www.tmp.com\"}"
	ret, _ := Extract([]byte(buf), "resultCode:\"([0-9]+)\"", "regex", nil)
	assert.Equal(t, "0000", fmt.Sprintf("%v", ret))
}

func TestHtmlExtractor(t *testing.T) {
	html := `
        <html>
            <head><title>fasdfs</title></head>
            <body>
                <table>
                    <tr>
                        <td>姓名</td>
                        <td>Liang Xiang</td>
                        <td>Hello World</td>
                    </tr>
                    <tr>
                        <td>年龄</td>
                        <td>30 <a>修改</a></td>
                    </tr>
                    <tr>
                        <td>机构</td>
                        <td>
                            <table>
                                <tr>
                                    <td>名称</td>
                                    <td>YYYY</td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    <tr>
                        <td>订单</td>
                        <td>
                            <table>
                                <tr>
                                    <td>1</td>
                                    <td>a</td>
                                </tr>
                                <tr>
                                    <td>2</td>
                                    <td>b</td>
                                </tr>
                                <tr>
                                    <td>3</td>
                                    <td>c</td>
                                </tr>
                                <tr>
                                    <td>4</td>
                                    <td>d</td>
                                </tr>
                                <tr>
                                    <td>5</td>
                                    <td>e</td>
                                </tr>
                                <tr>
                                    <td>6</td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </body>
        </html>
    `
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	s := doc.First()
	var config interface{}
	err := json.Unmarshal([]byte(`
        {
            "_root": "table",
            "name": "td:contains('姓名') + td",
            "age": "td:contains('年龄') + td&regex=([0-9]+)",
            "@dup0 company": {
                "_root": "td:contains('公司') + td table",
                "name": "td:contains('名称') + td"
            },
            "@dup1 company": {
                "_root": "td:contains('机构') + td table",
                "name": "td:contains('名称') + td"
            },
            "orders": {
                "_root": "td:contains('订单') + td tr",
                "_array": true,
                "@key td:nth-child(1)": "td:nth-child(2)"
            }
        }
    `), &config)
	if err != nil {
		t.Error(err)
		return
	}
	d := htmlExtract(s, config, nil)
	if m, ok := d.(map[string]interface{}); ok {
		assert.Equal(t, "Liang Xiang", fmt.Sprintf("%v", m["name"]))
		assert.Equal(t, "30", fmt.Sprintf("%v", m["age"]))
		if company, ok2 := m["company"]; ok2 {
			if mc, ok3 := company.(map[string]interface{}); ok3 {
				assert.Equal(t, "YYYY", fmt.Sprintf("%v", mc["name"]))
			} else {
				t.Error()
			}
		} else {
			t.Error()
		}
		if orders, ok2 := m["orders"]; ok2 {
			if ao, ok3 := orders.([]interface{}); ok3 {
				t.Log(ao)
				assert.Equal(t, 6, len(ao))
			} else {
				t.Error()
			}
		}
	} else {
		t.Error(d)
	}

	Extract([]byte(html), config, "html", nil)
}

func TestTaobaoHdc(t *testing.T) {
	body := `
        <span class=\"id-name\">支付宝个人认证</span>\r\n                              <span class=\"id-time\">2012-09-15</span>\r\n                          </a>\r\n\r\n                                        </span>\r\n    </p>\r\n    <div class=\"qualifications-dsr\">\r\n        <div class=\"qualifications\">
    `
	assert.Equal(t, "2012-09-15", regexExtract(body, ">([\\d]{4}-[\\d]{2}-[\\d]{2})</span>"))
}
