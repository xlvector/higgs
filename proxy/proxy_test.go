package proxy

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProxy(t *testing.T) {
	p := NewProxy("http://liangxiang:123456@127.0.0.1:8080")
	assert.Equal(t, "http://liangxiang:123456@127.0.0.1:8080", p.String())
}

func TestProxyManager(t *testing.T) {
	pm := NewProxyManager("")

	pm.AddProxy("socks5://127.0.0.1:80")
	assert.Equal(t, "socks5://127.0.0.1:80", pm.GetProxy().String())

	pm.BlockProxy(NewProxy("socks5://127.0.0.1:80"))
	if pm.GetProxy() != nil {
		t.Error()
	}

	pm.AddTmplProxy("hello", "http://a:b@127.0.0.1")
	assert.Equal(t, "http://a:b@127.0.0.1", pm.GetTmplProxy("hello").String())

	pm.BlockTmplProxy("hello", NewProxy("http://a:b@127.0.0.1"))
	if pm.GetTmplProxy("hello") != nil {
		t.Error()
	}
}
