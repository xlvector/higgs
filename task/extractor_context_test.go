package task

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/xlvector/higgs/context"
	"github.com/xlvector/higgs/extractor"
	"testing"
)

func TestExtractorWithContext(t *testing.T) {
	var html = `
        <html>
            <body>
                <div>Hello</div>
            </body>
        </html>
    `
	c := context.NewContext(nil)
	ret, err := extractor.Extract([]byte(html), "div||{{contains ._v \"Hello\"}}", "html", c)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, fmt.Sprintf("%v", ret), "true")
}
