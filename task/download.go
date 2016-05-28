package task

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/axgle/mahonia"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/casperjs"
	"github.com/xlvector/higgs/context"
	hproxy "github.com/xlvector/higgs/proxy"
	"github.com/xlvector/persistent-cookiejar"
	"golang.org/x/net/proxy"
	"golang.org/x/net/publicsuffix"
	"gopkg.in/redis.v2"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	DEFAULT_USERAGENT = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.80 Safari/537.36"
)

type DownloaderConfig struct {
	RedisHost         string
	RedisTimeout      time.Duration
	DisableOutputFile bool
}

type Downloader struct {
	Jar                 *cookiejar.Jar
	LastPage            []byte
	LastPageUrl         string
	LastPageStatus      int
	LastPageContentType string
	Client              *http.Client
	Context             *context.Context
	ExtractorResults    map[string]interface{}
	OutputFolder        string
	UploadFiles         []string
	RedisClient         *redis.Client
}

func NewHttpClientWithPersistentCookieJar() (*http.Client, *cookiejar.Jar) {
	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, err := cookiejar.New(&options)
	if err != nil {
		dlog.Warn("cookie jar error: %v", err)
		return nil, nil
	}

	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			dlog.Warn("CheckRedirect URL:%s", req.URL.String())
			return nil
		},
		Timeout: time.Second * 30,
	}, jar
}

func NewDownloader(cjs *casperjs.CasperJS, p *hproxy.Proxy, outFolder string, config *DownloaderConfig, pm *hproxy.ProxyManager) *Downloader {
	ret := &Downloader{
		Context:          context.NewContext(cjs, p, pm),
		OutputFolder:     outFolder,
		LastPage:         nil,
		ExtractorResults: make(map[string]interface{}),
	}
	if len(outFolder) > 0 {
		err := os.MkdirAll(outFolder, 0766)
		if err != nil {
			dlog.Error("fail to mkdir %s: %v", outFolder, err)
			return nil
		}
	}
	ret.Client, ret.Jar = NewHttpClientWithPersistentCookieJar()
	if p != nil {
		ret.SetProxy(p)
	}
	return ret
}

func (p *Downloader) SetCookie(b string) {
	p.Jar.ReadFrom(strings.NewReader(b))
}

func (p *Downloader) SaveCookie(fname string) error {
	body := &bytes.Buffer{}
	err := p.Jar.WriteTo(body)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fname, body.Bytes(), 0655)
}

func (p *Downloader) ExtractorResultString() string {
	b, _ := json.Marshal(p.ExtractorResults)
	return string(b)
}

func (p *Downloader) AddExtractorResult(data interface{}) {
	if m, ok := data.(map[string]interface{}); ok {
		for k, v := range m {
			pv, ok2 := p.ExtractorResults[k]
			if !ok2 {
				p.ExtractorResults[k] = v
			} else {
				a, ok3 := pv.([]interface{})
				b, ok4 := v.([]interface{})
				if ok4 {
					if !ok3 {
						dlog.Warn("prev value of %s is not array: %v", k, pv)
						p.ExtractorResults[k] = b
					} else {
						dlog.Info("append b[%d] to a[%d]", len(b), len(a))
						p.ExtractorResults[k] = append(a, b...)
					}
				} else {
					dlog.Warn("curr value of %s is not array: %v", k, v)
				}
			}
		}
	}
	b, _ := json.Marshal(p.ExtractorResults)
	p.Context.Set("_extractor", string(b))
}

func (self *Downloader) SetProxy(p *hproxy.Proxy) {
	transport := &http.Transport{
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: time.Second * 30,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MaxVersion:         tls.VersionTLS12,
			MinVersion:         tls.VersionTLS10,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_RC4_128_SHA,
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			},
		},
	}

	if p == nil {
		self.Client.Transport = transport
		return
	}

	if p.Type == "socks5" {
		var auth *proxy.Auth
		if len(p.Username) > 0 && len(p.Password) > 0 {
			auth = &proxy.Auth{
				User:     p.Username,
				Password: p.Password,
			}
		} else {
			auth = &proxy.Auth{}
		}
		forward := proxy.FromEnvironment()
		dialSocks5Proxy, err := proxy.SOCKS5("tcp", p.IP, auth, forward)
		if err != nil {
			dlog.Warn("SetSocks5 Error:%s", err.Error())
			return
		}
		transport.Dial = dialSocks5Proxy.Dial
	} else if p.Type == "http" {
		transport.Dial = func(netw, addr string) (net.Conn, error) {
			timeout := time.Second * 30
			deadline := time.Now().Add(timeout)
			c, err := net.DialTimeout(netw, addr, timeout)
			if err != nil {
				return nil, err
			}
			c.SetDeadline(deadline)
			return c, nil
		}
		proxyUrl, err := url.Parse(p.String())
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyUrl)
		}
	}

	self.Client.Transport = transport
	dlog.Warn("use proxy: %s", p.String())
}

func (s *Downloader) constructPage(resp *http.Response) error {
	defer resp.Body.Close()
	body := make([]byte, 1024)
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, _ := gzip.NewReader(resp.Body)
		defer reader.Close()
		for {
			buf := make([]byte, 1024)
			n, err := reader.Read(buf)
			if err != nil && err != io.EOF {
				return err
			}
			if n == 0 {
				break
			}
			body = append(body, buf...)
		}
	default:
		for {
			buf := make([]byte, 1024)
			n, err := resp.Body.Read(buf)
			if err != nil && err != io.EOF {
				return err
			}
			if n == 0 {
				break
			}
			body = append(body, buf...)
		}
	}
	s.LastPageUrl = resp.Request.URL.String()
	s.LastPage = body
	var charset string
	s.LastPageContentType, charset = decodeCharset(string(s.LastPage), resp.Header.Get("Content-Type"))

	if !strings.Contains(s.LastPageContentType, "image") && (strings.HasPrefix(charset, "gb") || strings.HasPrefix(charset, "GB")) {
		enc := mahonia.NewDecoder("gbk")
		cbody := []byte(enc.ConvertString(string(body)))
		s.LastPage = cbody
	}
	return nil
}

func decodeCharset(body, contentTypeHeader string) (string, string) {
	tks := strings.Split(contentTypeHeader, ";")
	var content_type, charset string

	if len(tks) == 1 {
		content_type = strings.ToLower(tks[0])
	}
	if len(tks) == 2 {
		kv := strings.Split(tks[1], "=")
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == "charset" {
			return strings.ToLower(tks[0]), strings.ToLower(kv[1])
		}
	}

	reg := regexp.MustCompile("meta[^<>]*[ ]{1}charset=\"([^\"]+)\"")
	result := reg.FindAllStringSubmatch(string(body), 1)
	if len(result) > 0 {
		group := result[0]
		if len(group) > 1 {
			charset = group[1]
		}
	}
	return content_type, charset
}

func (s *Downloader) Get(link string, header map[string]string) ([]byte, error) {
	dlog.Println(link)
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		dlog.Warn("new req error: %v", err)
		return nil, err
	}
	req.Header.Set("User-Agent", DEFAULT_USERAGENT)
	req.Header.Set("Referer", s.LastPageUrl)
	if header != nil {
		for name, value := range header {
			req.Header.Set(name, value)
		}
	}

	resp, err := s.Client.Do(req)

	if err != nil {
		dlog.Warn("do req error: %v", err)
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("nil resp")
	}
	s.LastPageStatus = resp.StatusCode
	err = s.constructPage(resp)
	if err != nil {
		return nil, err
	}
	s.UpdateCookieToContext(link)
	return s.LastPage, nil
}

func (s *Downloader) Post(link string, params map[string]string, header map[string]string) ([]byte, error) {
	uparams := url.Values{}
	for k, v := range params {
		uparams.Set(s.Context.Parse(k), s.Context.Parse(v))
	}
	dlog.Info("post paramter:%v", uparams)
	req, err := http.NewRequest("POST", link, strings.NewReader(uparams.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("User-Agent", DEFAULT_USERAGENT)
	req.Header.Set("Referer", s.LastPageUrl)
	if header != nil {
		for name, value := range header {
			req.Header.Set(name, value)
		}
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	err = s.constructPage(resp)
	if err != nil {
		return nil, err
	}
	s.UpdateCookieToContext(link)
	return s.LastPage, nil
}

func (s *Downloader) PostRaw(link string, data []byte, header map[string]string) ([]byte, error) {
	req, err := http.NewRequest("POST", link, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Set("User-Agent", DEFAULT_USERAGENT)
	req.Header.Set("Referer", s.LastPageUrl)
	if header != nil {
		for name, value := range header {
			req.Header.Set(name, value)
		}
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	err = s.constructPage(resp)
	if err != nil {
		return nil, err
	}
	s.UpdateCookieToContext(link)
	return s.LastPage, nil
}

func (s *Downloader) UpdateCookieToContext(link string) {
	ulink, _ := url.Parse(link)
	cs := s.Jar.Cookies(ulink)
	for _, c := range cs {
		s.Context.Set("cookie_"+c.Name, c.Value)
	}
}
