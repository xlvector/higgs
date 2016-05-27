package task

import (
	"crypto/rsa"
	"fmt"
	"github.com/xlvector/dama2"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/casperjs"
	"github.com/xlvector/higgs/cmd"
	"github.com/xlvector/higgs/config"
	"github.com/xlvector/higgs/flume"
	hproxy "github.com/xlvector/higgs/proxy"
	"github.com/xlvector/higgs/util"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

const (
	STATUS_START  = "started"
	STATUS_FINISH = "finished"
	STATUS_FAIL   = "failed"
)

type TaskCmd struct {
	id           string
	tmpl         string
	userName     string
	userId       string
	passWord     string
	path         string
	message      chan *cmd.Output
	input        chan map[string]string
	args         map[string]string
	privateKey   *rsa.PrivateKey
	downloader   *Downloader
	task         *Task
	casperJS     *casperjs.CasperJS
	dama2Client  *dama2.Dama2Client
	flumeClient  *flume.Flume
	finished     bool
	proxy 	     *hproxy.Proxy
}

type TaskCmdFactory struct {
	taskManager  *TaskManager
	proxyManager *hproxy.ProxyManager
}

func NewTaskCmdFactory(tm *TaskManager, pm *hproxy.ProxyManager) *TaskCmdFactory {
	return &TaskCmdFactory{
		taskManager:  tm,
		proxyManager: pm,
	}
}

func (s *TaskCmdFactory) CreateCommand(params url.Values) cmd.Command {
	tmpl := params.Get("tmpl")
	if len(tmpl) == 0 {
		dlog.Warn("find empty tmpl")
		return nil
	}
	task := s.taskManager.Get(tmpl)
	if task == nil {
		return nil
	}
	if !task.DisableOutPubKey {
		pk, err := util.GenerateRSAKey()
		if err != nil {
			dlog.Fatalln("fail to generate rsa key", err)
		}
		return s.createCommandWithPrivateKey(params, task, pk)
	}
	dlog.Println("begin create command")
	return s.createCommandWithPrivateKey(params, task, nil)
}

func (s *TaskCmdFactory) CreateCommandWithPrivateKey(params url.Values, pk *rsa.PrivateKey) cmd.Command {
	tmpl := params.Get("tmpl")
	if len(tmpl) == 0 {
		return nil
	}
	task := NewTask(tmpl + ".json")
	if task == nil {
		return nil
	}
	return s.createCommandWithPrivateKey(params, task, pk)
}

func (s *TaskCmdFactory) genId(tmpl string) string {
	tm := time.Now().Format("2006|01|02")
	return fmt.Sprintf("%s|%s|%d", tmpl, tm, time.Now().UnixNano())
}

func (s *TaskCmdFactory) genFolderById(id string) string {
	tks := strings.Split(id, "|")
	return config.Instance.OutputRoot + strings.Join(tks, "/")
}

func (s *TaskCmdFactory) createCommandWithPrivateKey(params url.Values, task *Task, pk *rsa.PrivateKey) cmd.Command {
	tmpl := params.Get("tmpl")
	ret := &TaskCmd{
		id:          s.genId(tmpl),
		tmpl:        tmpl,
		userName:    "",
		userId:      params.Get("userid"),
		passWord:    "",
		message:     make(chan *cmd.Output, 5),
		input:       make(chan map[string]string, 5),
		args:        make(map[string]string),
		task:        task,
		dama2Client: dama2.NewDama2Client(config.Instance.Captcha.Key),
		finished:    false,
	}

	if config.Instance.HasFlume() {
		ret.flumeClient = flume.NewFlume(config.Instance.Flume.Host, config.Instance.Flume.Port)
	}

	ret.privateKey = pk
	ret.path = s.genFolderById(ret.id)
	var p *hproxy.Proxy
	if s.proxyManager != nil {
		p = s.proxyManager.GetTmplProxy(tmpl)
	}
	ret.proxy = p
	if len(task.CasperjsScript) > 0 {
		ret.casperJS, _ = casperjs.NewCasperJS(ret.path, "./etc/casperjs/"+task.CasperjsScript, "", "")
		go ret.casperJS.Run()
	}

	outFolder := ret.path
	if task.DisableOutputFolder {
		outFolder = ""
	}
	ret.downloader = NewDownloader(ret.casperJS, p, outFolder, &DownloaderConfig{
		RedisHost:    config.Instance.Redis.Host,
		RedisTimeout: time.Duration(config.Instance.Redis.Timeout),
	},   s.proxyManager)

	dlog.Warn("output folder: %s", ret.downloader.OutputFolder)
	ret.downloader.Context.Set("_id", ret.GetId())
	ret.downloader.Context.Set("tmpl", tmpl)
	go ret.run()
	return ret
}

func (p *TaskCmd) GetId() string {
	return p.id
}

func (p *TaskCmd) Finished() bool {
	return p.finished
}

func (p *TaskCmd) SetInputArgs(input map[string]string) {

	if p.Finished() {
		dlog.Warn("finished")
	}
	p.input <- input
}

func (p *TaskCmd) GetMessage() *cmd.Output {
	return <-p.message
}

func (p *TaskCmd) getUserName() string {
	if len(p.userName) == 0 {
		if v, ok := p.downloader.Context.Get("username"); ok {
			p.userName = fmt.Sprintf("%v", v)
		}
	}
	return p.userName
}

func (p *TaskCmd) readInputArgs(key string) string {
	args := <-p.input
	for k, v := range args {
		if k == "username" {
			p.userName = v
		}

		if k == "password" {
			p.passWord = v
		}

		p.args[k] = v
	}
	if val, ok := p.args[key]; ok {
		return val
	}

	dlog.Warn("%s need parameter %s", p.GetId(), key)
	message := &cmd.Output{
		Id:        p.GetArgsValue("id"),
		NeedParam: key,
		Status:    cmd.NEED_PARAM,
	}
	dlog.Warn("%s need param:%s", p.GetId(), key)
	p.message <- message
	return ""
}

func (p *TaskCmd) GetArgsValue(key string) string {
	if val, ok := p.args[key]; ok {
		dlog.Info("%s successfully get args value for key:%s, %s", p.GetId(), val, key)
		return val
	}
	for {
		val := p.readInputArgs(key)
		if len(val) != 0 {
			dlog.Info("%s successfully get args value:%s", p.GetId(), val)
			return val
		}
	}
}

func (p *TaskCmd) Successed() bool {
	return true
}

func (p *TaskCmd) Close() bool {
	defer func() {
		if err := recover(); err != nil {
			dlog.Warn("%s Close Error:%v", p.GetId(), err)
		}
	}()
	close(p.message)
	close(p.input)
	return true
}

func (p *TaskCmd) OutputPublicKey() {
	if p.task.DisableOutPubKey == false {
		message := &cmd.Output{
			Id:     p.GetArgsValue("id"),
			Status: cmd.OUTPUT_PUBLICKEY,
			Data:   string(util.PublicKeyString(&p.privateKey.PublicKey)),
		}
		p.message <- message
	}
}

func (p *TaskCmd) Goto() (map[string]int, map[string]int) {
	gotoMap := make(map[string]int)
	retry := make(map[string]int)

	for k, step := range p.task.Steps {
		if len(step.Tag) > 0 {
			gotoMap[step.Tag] = k - 1
			retry[step.Tag] = 0
		}
	}
	return gotoMap, retry
}

func (p *TaskCmd) run() {
	defer func() {
		if err := recover(); err != nil {
			dlog.Warn("run error:%v", err)
			debug.PrintStack()
		}
	}()
	p.downloader.Context.Set(p.tmpl, "exist")
	dlog.Info("%s begin run cmd:%s", p.GetId(), p.tmpl)

	p.finished = false
	p.OutputPublicKey()

	gotoMap, retry := p.Goto()

	c := 0
	for {
		if c >= len(p.task.Steps) {
			break
		}

		step := p.task.Steps[c]
		time.Sleep(time.Duration(rand.Int63n(300)) * time.Millisecond)

		if len(step.NeedParam) > 0 {
			tks := strings.Split(step.NeedParam, ",")
			for _, tk := range tks {
				_, ok := p.downloader.Context.Get(tk)
				if !ok {
					val := p.GetArgsValue(tk)
					delete(p.args, tk)
					if tk == "password" {
						val = util.DecodePassword(val, p.privateKey)
					}
					p.downloader.Context.Set(tk, val)
				}
			}
		}

		if p.proxy == nil {
			data := "{\"tmpl\":\""+p.tmpl+"\"}"
			msg := &cmd.Output{
				Status: cmd.TMPL_BLOCK,
				Id:	p.GetArgsValue("id"),
				Data:	data,
				//Data:	p.downloader.Context.Parse(step.Message["data"]),
			}

			p.message <- msg
			dlog.Info("output msg: %v", msg)

			p.finished = true
			return
		}

		if !step.passCondition(p.downloader.Context) {
			dlog.Warn("skip step %d", c)
			c++
			continue
		}

		err := step.Do(p.downloader, p.dama2Client, p.casperJS)
		if nil != err {
			dlog.Warn("%s downloader dostep fail: %v", p.GetId(), err)
		}

		if !p.task.DisableOutputFolder {
			dlog.Println("begin save cookie")
			err = p.downloader.SaveCookie(p.downloader.OutputFolder + "/task_cookies.json")
			if nil != err {
				dlog.Warn("save cookie fail: %v", err)
			}
		}

		if p.downloader.LastPageStatus/100 == 4 || p.downloader.LastPageStatus/100 == 5 {
			msg := &cmd.Output{
				Status: cmd.WRONG_RESPONSE,
				Id:	p.GetArgsValue("id"),
				Data:	p.downloader.Context.Parse(step.Message["data"]),
			}

			p.message <- msg
			dlog.Info("output msg: %v", msg)

			p.finished = true
			return
		}
		if step.Message != nil && len(step.Message) > 0 {
			msg := &cmd.Output{
				Status: step.Message["status"],
				Id:     p.GetArgsValue("id"),
				Data:   p.downloader.Context.Parse(step.Message["data"]),
			}

			if needParam, ok := step.Message["need_param"]; ok {
				msg.NeedParam = needParam
			}

			p.message <- msg

			if msg.Status == cmd.FAIL || msg.Status == cmd.FINISH_FETCH_DATA {
				p.finished = true
				return
			}
		}

		if action := step.GetAction(p.downloader.Context); action != nil {
			dlog.Info("fire action %v", action)
			actionInfo := action.FullInfo(p.downloader.Context)
			if action.Message != nil {
				msg := &cmd.Output{
					Status:    action.Message["status"],
					Id:        p.GetArgsValue("id"),
					NeedParam: action.Message["need_param"],
					Data:      actionInfo,
				}

				p.message <- msg
				if msg.Status == cmd.FAIL || msg.Status == cmd.FINISH_FETCH_DATA {
					p.finished = true
					return
				}
			}

			if len(action.Goto) > 0 {
				nr, ok := retry[action.Goto]
				if !ok {
					retry[action.Goto] = 1
				} else {
					retry[action.Goto] = nr + 1
				}

				dlog.Warn("%s retry count :%d", p.GetId(), nr)
				maxRetry := 1
				if step.Retry != nil {
					maxRetry = step.Retry.MaxTimes
				}

				if step.Retry != nil && step.Retry.ContinueThen == false && ok && nr >= maxRetry {
					msg := &cmd.Output{
						Status: cmd.FAIL,
						Id:     p.GetArgsValue("id"),
						Data:   actionInfo,
					}
					p.message <- msg
					dlog.Warn("%s Status:%s", p.GetId(), "retry fail "+step.Page)
					return
				} else if ok && nr < maxRetry {
					c, ok = gotoMap[action.Goto]
					dlog.Info("goto step %d with tag %s", c, action.Goto)
					if !ok {
						dlog.Fatalln(p.GetId(), " can not find goto tag ", action.Goto)
					}
				}
			}

			for _, d := range action.DeleteContext {
				delete(p.args, d)
				p.downloader.Context.Del(d)
			}
		}
		c++
	}

	if !p.task.DisableOutputFolder {
		path := p.downloader.OutputFolder + "/ExtractorInfo.json"
		saveFile, err := os.Create(path)
		if err == nil {
			saveFile.WriteString(p.downloader.ExtractorResultString())
			saveFile.Close()
		}
		err = util.Tarit(p.downloader.OutputFolder, strings.TrimRight(p.downloader.OutputFolder, "/")+".tar")
		if err != nil {
			dlog.Warn("tar output from %s failed: %v", p.downloader.OutputFolder, err)
		} else if p.flumeClient != nil {
			fb, _ := ioutil.ReadFile(strings.TrimRight(p.downloader.OutputFolder, "/") + ".tar")
			p.flumeClient.Send(p.tmpl, fb)
			udLink := util.UploadFile(strings.TrimRight(p.downloader.OutputFolder, "/")+".tar", USERDATA_BUCKET)
			p.downloader.AddExtractorResult(map[string]interface{}{
				"data_link": udLink,
			})
		}
	}

	message := &cmd.Output{
		Status: cmd.FINISH_FETCH_DATA,
		Id:     p.GetArgsValue("id"),
		Data:   p.downloader.ExtractorResultString(),
	}

	p.message <- message
	p.finished = true
}
