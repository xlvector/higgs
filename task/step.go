package task

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/SKatiyar/qr"
	"github.com/xlvector/dama2"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/casperjs"
	"github.com/xlvector/higgs/config"
	"github.com/xlvector/higgs/context"
	"github.com/xlvector/higgs/extractor"
	"github.com/xlvector/higgs/util"
	"io/ioutil"
	"strconv"
	"time"
	"net/url"
	"strings"
)

type Require struct {
	File string `json:"file"`
	From string `json:"from"`
	To   string `json:"to"`
}

type Retry struct {
	MaxTimes     int  `json:"max_times"`
	ContinueThen bool `json:"continue_then"`
}

type QRCodeImage struct {
	Src        string `json:"src"`
	ContextKey string `json:"context_key"`
}

type Captcha struct {
	CodeType   string `json:"code_type"`
	ImgFormat  string `json:"img_format"`
	ContextKey string `json:"context_key"`
}

type UploadImage struct {
	ContextKey string `json:"context_key"`
	Format     string `json:"format"`
	Base64Src  string `json:"base64_src"`
}

func (p *UploadImage) Filename() string {
	return p.ContextKey + "." + p.Format
}

type Step struct {
	Require         *Require               `json:"require"`
	Tag             string                 `json:"tag"`
	Retry           *Retry                 `json:"retry"`
	CookieJar       string                 `json:"cookiejar"`
	Condition       string                 `json:"condition"`
	NeedParam       string                 `json:"need_param"`
	Page            string                 `json:"page"`
	Method          string                 `json:"method"`
	Header          map[string]string      `json:"header"`
	Params          map[string]string      `json:"params"`
	Actions         []*Action              `json:"actions"`
	JsonPostBody    interface{}            `json:"json_post_body"`
	UploadImage     *UploadImage           `json:"upload_image"`
	Captcha         *Captcha               `json:"captcha"`
	QRcodeImage     *QRCodeImage           `json:"qrcode_image"`
	DocType         string                 `json:"doc_type"`
	OutputFilename  string                 `json:"output_filename"`
	ContextOpers    []string               `json:"context_opers"`
	ExtractorSource string                 `json:"extractor_source"`
	Extractor       map[string]interface{} `json:"extractor"`
	Sleep           int                    `json:"sleep"`
	Message         map[string]string
}

func (s *Step) getPageUrls(c *context.Context) string {
	return c.Parse(s.Page)
}

func (s *Step) getParams(c *context.Context) map[string]string {
	ret := make(map[string]string)
	for k, v := range s.Params {
		ret[c.Parse(k)] = c.Parse(v)
	}
	return ret
}

func (s *Step) addContextOutputs(c *context.Context) {
	for _, co := range s.ContextOpers {
		dlog.Info("parse %s", co)
		c.Parse(co)
	}
}

func (s *Step) extract(body []byte, d *Downloader) {
	if s.Extractor == nil || len(s.Extractor) == 0 {
		return
	}
	if len(s.ExtractorSource) > 0 {
		body = []byte(d.Context.Parse(s.ExtractorSource))
	}
	ret, err := extractor.Extract(body, s.Extractor, s.DocType, d.Context)
	if err != nil {
		dlog.Warn("extract error of %v: %v", s.Extractor, err)
		return
	}
	d.AddExtractorResult(ret)
}

func (s *Step) getHeader(c *context.Context) map[string]string {
	ret := make(map[string]string)
	for k, v := range s.Header {
		ret[c.Parse(k)] = c.Parse(v)
	}
	return ret
}

func (s *Step) getRawPostData() []byte {
	b, _ := json.Marshal(s.JsonPostBody)
	return b
}

func (s *Step) download(d *Downloader) ([]byte, error) {
	page := s.getPageUrls(d.Context)
	if strings.Contains(page,"%"){
		page,_ = url.QueryUnescape(page)
	}
	dlog.Info("download %s", page)
	d.UpdateCookieToContext(page)
	if len(s.Method) == 0 || s.Method == "GET" {
		return d.Get(page, s.getHeader(d.Context))
	} else if s.Method == "POST" {
		return d.Post(page, s.getParams(d.Context), s.getHeader(d.Context))
	} else if s.Method == "POSTJSON" {
		return d.PostRaw(page, s.getRawPostData(), s.getHeader(d.Context))
	}
	return nil, errors.New("unsupported method: " + s.Method)
}

func (s *Step) passCondition(c *context.Context) bool {
	if len(s.Condition) == 0 {
		return true
	}
	return c.Parse(s.Condition) == "true"
}

func (s *Step) GetAction(c *context.Context) *Action {
	for _, f := range s.Actions {
		if f.IsFire(c) {
			return f
		}
	}
	return nil
}

func (s *Step) GetOutputFilename(c *context.Context) string {
	if len(s.OutputFilename) == 0 {
		return ""
	}
	return c.Parse(s.OutputFilename)
}

func (s *Step) procUploadImage(body []byte, d *Downloader) error {
	b := body
	if len(s.UploadImage.Base64Src) > 0 {
		bsrc := d.Context.Parse(s.UploadImage.Base64Src)
		b, _ = base64.StdEncoding.DecodeString(bsrc)
	}
	imgLink, err := util.UploadBody(b, d.OutputFolder+"/"+s.UploadImage.Filename(), CAPTCHA_BUCKET)
	if err != nil {
		dlog.Warn("upload image fail: %v", err)
		return err
	}
	dlog.Info("upload image to %s", imgLink)
	d.Context.Set(s.UploadImage.ContextKey, imgLink)
	return nil
}

func (s *Step) Do(d *Downloader, dm *dama2.Dama2Client, cas *casperjs.CasperJS) error {
	if !s.passCondition(d.Context) {
		return nil
	}

	if len(s.CookieJar) > 0 {
		d.SetCookie(d.Context.Parse(s.CookieJar))
	}

	body := []byte{}
	if len(s.Page) > 0 {
		var err error
		body, err = s.download(d)
		if err != nil {
			return err
		}
	}

	//output file name should calculated before context operations
	out := s.GetOutputFilename(d.Context)
	d.Context.Set("_body", string(body))
	s.addContextOutputs(d.Context)
	s.extract(body, d)

	if len(out) > 0 {
		dlog.Info("write file %s to %s", out, d.OutputFolder+"/"+out)
		err := ioutil.WriteFile(d.OutputFolder+"/"+out, body, 0655)
		if err != nil {
			dlog.Warn("write file failed: %v", err)
		}
	}

	if s.UploadImage != nil {
		s.procUploadImage(body, d)
	}

	if s.QRcodeImage != nil {
		qc, qerr := qr.Encode(d.Context.Parse(s.QRcodeImage.Src), qr.M)
		if qerr != nil {
			dlog.Warn("Encode Qrcode Err:%s", qerr.Error())
		} else {
			png := qc.PNG()
			uploadUrl, err := util.UploadBody(png, d.OutputFolder+"/qrcode.png", CAPTCHA_BUCKET)
			if err != nil {
				dlog.Warn("upload image err:%s", err.Error())
			}
			d.Context.Set(s.QRcodeImage.ContextKey, uploadUrl)
		}
	}

	if s.Captcha != nil && dm != nil {
		ct, _ := strconv.Atoi(s.Captcha.CodeType)
		cret, err := dm.Captcha(body, s.Captcha.ImgFormat, ct, config.Instance.Captcha.AppId, config.Instance.Captcha.Username, config.Instance.Captcha.Password)
		if err != nil {
			dlog.Warn("decode captcha error : %v", err)
		}
		d.Context.Set(s.Captcha.ContextKey, cret)
	}

	if s.Sleep > 0 {
		time.Sleep(time.Duration(s.Sleep) * time.Second)
	}
	return nil
}
