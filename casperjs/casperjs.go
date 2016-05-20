package casperjs

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"github.com/xlvector/dlog"
	"io/ioutil"
	"os/exec"
	"strings"
)

type CasperJS struct {
	Path       string
	Cmd        *exec.Cmd
	output     *bufio.Reader
	input      *bufio.Writer
	inputChan  chan []byte
	outputChan chan []byte
}

func NewCasperJS(path, script, proxyServer, proxyType string) (*CasperJS, error) {
	ret := &CasperJS{
		Path:       path,
		inputChan:  make(chan []byte, 10),
		outputChan: make(chan []byte, 10),
	}
	if len(proxyServer) == 0 {
		ret.Cmd = exec.Command("casperjs", script,
			"--ignore-ssl-errors=true",
			"--web-security=no",
			"--cookies-file="+path+"/cookie.txt",
			"--context="+path)
	} else {
		authHost := strings.Split(proxyServer, "@")
		if len(authHost) == 1 {
			ret.Cmd = exec.Command("casperjs", script,
				"--ignore-ssl-errors=true",
				"--web-security=no",
				"--cookies-file="+path+"/cookie.txt",
				"--proxy="+proxyServer, "--proxy-type="+proxyType,
				"--context="+path)
		} else {
			ret.Cmd = exec.Command("casperjs", script,
				"--ignore-ssl-errors=true",
				"--web-security=no",
				"--cookies-file="+path+"/cookie.txt",
				"--proxy="+authHost[1], "--proxy-type="+proxyType,
				"--proxy-auth="+authHost[0],
				"--context="+path)
		}
	}
	stdin, err := ret.Cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := ret.Cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ret.input = bufio.NewWriter(stdin)
	ret.output = bufio.NewReader(stdout)
	return ret, nil
}

func (p *CasperJS) Start() error {
	return p.Cmd.Start()
}

func (p *CasperJS) ReadChan() []byte {
	ret := <-p.outputChan
	return ret
}

func (p *CasperJS) WriteChan(b []byte) {
	p.inputChan <- b
}

func (p *CasperJS) Read() (string, error) {
	return p.output.ReadString('\n')
}

func (p *CasperJS) Write(line string) error {
	_, err := p.input.WriteString(line)
	if err != nil {
		return err
	}
	_, err = p.input.WriteRune('\n')
	if err != nil {
		return err
	}
	return p.input.Flush()
}

type Cookie struct {
	Domain   string `json:"domain"`
	Expiry   int64  `json:"expiry"`
	Httponly bool   `json:"httponly"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	Value    string `json:"value"`
}

func (c *Cookie) getDomain() string {
	return strings.Trim(c.Domain, ".")
}

func (c *Cookie) key() string {
	return c.getDomain() + ";/;" + c.Name
}

func (c *Cookie) HiggsCookie() map[string]interface{} {
	return map[string]interface{}{
		"Domain":   c.getDomain(),
		"Name":     c.Name,
		"Value":    c.Value,
		"Path":     c.Path,
		"Secure":   c.Secure,
		"HttpOnly": c.Httponly,
	}
}

func (p *CasperJS) ConvertCookie(cookies []*Cookie) []byte {
	ret := make(map[string]map[string]interface{})
	for _, c := range cookies {
		_, ok := ret[c.getDomain()]
		if !ok {
			ret[c.getDomain()] = make(map[string]interface{})
		}
		_, ok2 := ret[c.getDomain()][c.key()]
		if !ok2 {
			ret[c.getDomain()][c.key()] = c.HiggsCookie()
		}
	}
	b, _ := json.Marshal(ret)
	return b
}

func (p *CasperJS) Run() error {
	err := p.Start()
	if err != nil {
		return err
	}
	for {
		line, err := p.Read()
		if err != nil {
			dlog.Warn("read line failed: %v", err)
			break
		}
		line = strings.TrimSpace(line)

		if line == "finish" {
			break
		}

		if strings.HasPrefix(line, "fail") {
			p.outputChan <- []byte(line)
			break
		}

		if line == "randcode" {
			b, err := ioutil.ReadFile(p.Path + "/casperjs_randcode.png")
			if err != nil {
				dlog.Warn("read randcode error: %v", err)
				break
			}
			imgb64 := base64.StdEncoding.EncodeToString(b)
			p.outputChan <- []byte(imgb64)

			randcode := <-p.inputChan
			p.Write(string(randcode))
			continue
		}

		if strings.HasPrefix(line, "cookie") {
			line = strings.TrimPrefix(line, "cookie")
			cookies := []*Cookie{}
			json.Unmarshal([]byte(line), &cookies)
			p.outputChan <- p.ConvertCookie(cookies)
			break
		}

		if strings.HasPrefix(line, "need") || strings.HasPrefix(line, "skip") {
			p.outputChan <- []byte(line)
			continue
		}

		b := <-p.inputChan
		err = p.Write(string(b))
		if err != nil {
			dlog.Warn("write line failed: %v", err)
			break
		}
	}
	p.Cmd.Process.Wait()
	p.Cmd.Process.Kill()
	//TODO: read cookie
	b, err := ioutil.ReadFile(p.Path + "/cookie.txt")
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > 2 {
		p.outputChan <- []byte(lines[1])
	}
	return nil
}
