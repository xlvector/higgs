package proxy

import (
	"encoding/json"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/config"
	"gopkg.in/redis.v3"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//If redis is setup, use proxies in redis, otherwise, use proxies in ProxyConfig.Proxies
type ProxyConfig struct {
	Proxies map[string][]string `json:"proxies"`
	Tmpls   map[string]string   `json:"tmpls"`
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		Proxies: make(map[string][]string),
		Tmpls:   make(map[string]string),
	}
}

type Proxy struct {
	IP        string
	Type      string
	Username  string
	Password  string
	BlockTime time.Time
}

func NewProxy(buf string) *Proxy {
	typeOthers := strings.SplitN(buf, "://", 2)
	if len(typeOthers) != 2 {
		return nil
	}
	authOthers := strings.SplitN(typeOthers[1], "@", 2)
	if len(authOthers) == 1 {
		return &Proxy{
			IP:        authOthers[0],
			Type:      typeOthers[0],
			Username:  "",
			Password:  "",
			BlockTime: time.Now(),
		}
	} else if len(authOthers) == 2 {
		userPwd := strings.SplitN(authOthers[0], ":", 2)
		return &Proxy{
			IP:        authOthers[1],
			Type:      typeOthers[0],
			Username:  userPwd[0],
			Password:  userPwd[1],
			BlockTime: time.Now(),
		}
	}
	return nil
}

func (p *Proxy) String() string {
	ret := p.Type + "://"
	if len(p.Username) > 0 {
		ret += p.Username + ":" + p.Password + "@"
	}
	ret += p.IP
	return ret
}

func (p *Proxy) Available() bool {
	if strings.Contains(p.IP, "127.0.0.1") {
		return true
	}
	conn, err := net.DialTimeout("tcp", p.IP, time.Second*5)
	if err != nil {
		//util.SlackMessage(config.Instance.SlackApi, "#crawler", "higgs", "proxy "+p.IP+" is not available")
		return false
	}
	conn.Close()
	return true
}

func (p *Proxy) IsBlock() bool {
	return p.BlockTime.Sub(time.Now()).Seconds() > 0.0
}

const (
	DEFAULT_TMPL = "default"
)

type ProxyManager struct {
	tmplProxies map[string]map[string]*Proxy
	proxyConfig *ProxyConfig
	client      *redis.Client
	lock        *sync.RWMutex
}

func NewProxyManager(conf string) *ProxyManager {
	ret := ProxyManager{
		tmplProxies: make(map[string]map[string]*Proxy),
		lock:        &sync.RWMutex{},
	}
	ret.client = redis.NewClient(&redis.Options{
		Addr:        config.Instance.Redis.Host,
		DialTimeout: time.Duration(config.Instance.Redis.Timeout) * time.Second,
	})
	if len(conf) > 0 {
		b, err := ioutil.ReadFile(conf)
		if err != nil {
			dlog.Fatal("read proxy conf failed: %v", err)
		}
		err = json.Unmarshal(b, &ret.proxyConfig)
		if err != nil {
			dlog.Fatal("fail to unmarshal proxy conf: %v", err)
		}
		ret.tmplProxies = ret.genTmplProxiesFromConfig(ret.proxyConfig)
		go ret.checkProxies()
	}
	return &ret
}

func (p *ProxyManager) genTmplProxiesFromConfig(pc *ProxyConfig) map[string]map[string]*Proxy {
	ret := make(map[string]map[string]*Proxy)
	for tmpl, pn := range pc.Tmpls {
		ret[tmpl] = make(map[string]*Proxy)
		if ps, ok := pc.Proxies[pn]; ok {
			dlog.Info("add %d proxys to tmpl %s", len(ps), tmpl)
			for _, p := range ps {
				py := NewProxy(p)
				if py.Available() {
					ret[tmpl][p] = NewProxy(p)
				}
			}
		}
	}
	return ret
}

func (p *ProxyManager) refreshProxiesFromRedis() {
	_, err := p.client.Ping().Result()
	if err != nil {
		dlog.Warn("redis can not be connected: %v", err)
		return
	}
	pc := NewProxyConfig()
	pc.Tmpls = p.proxyConfig.Tmpls
	for pname, oldPs := range p.proxyConfig.Proxies {
		ps, err := p.client.LRange("proxy_"+pname, 0, 20).Result()
		if err != nil || len(ps) == 0 {
			pc.Proxies[pname] = oldPs
			continue
		}
		pc.Proxies[pname] = ps
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.proxyConfig = pc
	p.tmplProxies = p.genTmplProxiesFromConfig(pc)
}

func (p *ProxyManager) checkProxies() {
	tc := time.NewTicker(time.Second * 30)
	for _ = range tc.C {
		dlog.Println("begin check proxies")
		p.refreshProxiesFromRedis()
		for _, ps := range p.tmplProxies {
			for _, py := range ps {
				if py.IsBlock() {
					continue
				}
				if !py.Available() {
					dlog.Warn("proxy %s is not available", py.String())
					py.BlockTime = time.Now().Add(time.Minute * 10)
				}
			}
		}
	}
}

func (p *ProxyManager) getProxyFromMap(ps map[string]*Proxy) *Proxy {
	for _, v := range ps {
		if v.IsBlock() {
			dlog.Warn("proxy %s is blocked", v.String())
			continue
		}
		if !v.Available() {
			dlog.Warn("proxy %s is not available", v.String())
			continue
		}
		return v
	}
	return nil
}

func (p *ProxyManager) GetProxy() *Proxy {
	return p.GetTmplProxy(DEFAULT_TMPL)
}

func (p *ProxyManager) GetTmplProxy(tmpl string) *Proxy {
	p.lock.RLock()
	p.lock.RUnlock()
	if ps, ok := p.tmplProxies[tmpl]; ok {
		return p.getProxyFromMap(ps)
	}
	return nil
}

func (p *ProxyManager) AddProxy(proxyStr string) {
	p.AddTmplProxy(DEFAULT_TMPL, proxyStr)
}

func (p *ProxyManager) AddTmplProxy(tmpl, proxyStr string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	proxy := NewProxy(proxyStr)
	if proxy == nil {
		return
	}
	if ps, ok := p.tmplProxies[tmpl]; ok {
		ps[proxyStr] = proxy
	} else {
		p.tmplProxies[tmpl] = make(map[string]*Proxy)
		p.tmplProxies[tmpl][proxyStr] = proxy
	}
}

func (p *ProxyManager) BlockProxy(proxy *Proxy) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if proxy == nil {
		return
	}
	pstr := proxy.String()
	for _, ps := range p.tmplProxies {
		if py, ok := ps[pstr]; ok {
			py.BlockTime = time.Now().Add(time.Minute * 10)
		}
	}
}

func (p *ProxyManager) BlockTmplProxy(tmpl string, proxy *Proxy) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if proxy == nil {
		return
	}
	pstr := proxy.String()
	if ps, ok := p.tmplProxies[tmpl]; ok {
		if py, ok2 := ps[pstr]; ok2 {
			py.BlockTime = time.Now().Add(time.Minute * 10)
		}
	}
}

func (p *ProxyManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.lock.Lock()
	defer p.lock.Unlock()
	b, _ := json.Marshal(p.tmplProxies)
	w.Header().Set("Content-Type", "application/json; encoding=UTF-8")
	w.Write(b)
}
