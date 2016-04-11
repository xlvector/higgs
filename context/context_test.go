package context

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTemplate(t *testing.T) {
	c := NewContext(nil)
	t.Log(c.Parse("{{nowTimestamp}}"))
	t.Log(c.Parse("{{daysAgo 1 \"2006-01-02\"}}"))
	t.Log(c.Parse("{{nowMillTimestamp}}"))

	c.Set("hello", "world")
	assert.Equal(t, "world", c.Parse("{{.hello}}"))

	assert.Equal(t, "true", c.Parse("{{lt 1 2}}"))
	assert.Equal(t, "false", c.Parse("{{lt 2 1}}"))
	assert.Equal(t, "true", c.Parse("{{eq 1 1}}"))
	assert.Equal(t, "true", c.Parse("{{eq .hello \"world\"}}"))

	assert.Equal(t, "true", c.Parse("{{and (eq .hello \"world\") (lt 1 2)}}"))

	c.Set("a", []string{"1", "2", "3"})
	assert.Equal(t, "http://hello.com/?world=1|http://hello.com/?world=2|http://hello.com/?world=3|", c.Parse("{{range .a}}http://hello.com/?world={{.}}|{{end}}"))

	assert.Equal(t, "%E6%AD%A3", c.Parse("{{urlquery \"正\"}}"))

	c.Set("html", `
        <html>
            <body>
                <div id="a">
                    <span>001</span>
                    <span>002</span>
                </div>
            </body>
        </html>
        `)

	assert.Equal(t, "001", c.Parse("{{extractHtml .html \"#a span:contains('001')\" | set \"html_ret\"}}"))
	if v, ok := c.Get("html_ret"); ok {
		assert.Equal(t, "001", v)
	}
	c.Set("json", `
        {
            "a": [
                {
                    "b": "001"
                },
                {
                    "b": "002"
                }
            ]
        }
        `)
	assert.Equal(t, "002", c.Parse("{{extractJson .json \"a[1].b\" | set \"json_ret\"}}"))
	if v, ok := c.Get("json_ret"); ok {
		assert.Equal(t, "002", v)
	}

	c.Set("_body", "jQuery172047437679511494935_1455015551115({resultCode:\"0000\",redirectURL:\"http://www.tmp.com\"}")
	assert.Equal(t, "0000", c.Parse("{{extractRegex ._body \"resultCode:\\\"([0-9]+)\\\"\"}}"))

	t.Log(c.Parse("{{lastDayOfMonthAgo 0 \"2006-01-02\"}}"))
	t.Log(c.Parse("{{firstDayOfMonthAgo 0 \"2006-01-02\"}}"))

	assert.Equal(t, "true", c.Parse("{{or (contains ._body \"result\") (contains ._body \"haha\") (contains ._body \"com\")}}"))
	assert.Equal(t, "false", c.Parse("{{and (contains ._body \"result\") (contains ._body \"haha\") (contains ._body \"com\")}}"))

	assert.Equal(t, "20140302", c.Parse("{{changeTimeFormat \"2014-03-02\" \"2006-01-02\" \"20060102\"}}"))
}
