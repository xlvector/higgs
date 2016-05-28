package task

import (
	"encoding/json"
	"github.com/xlvector/dlog"
	"io/ioutil"
)

const (
	CAPTCHA_BUCKET  = "crawlercaptchas"
	USERDATA_BUCKET = "crawleruserdata"
)

type Task struct {
	Steps               []*Step `json:"steps"`
	DisableOutPubKey    bool    `json:"disable_out_pub_key"`
	DisableOutputFolder bool    `json:"disable_output_folder"`
	CasperjsScript      string  `json:"casperjs_script"`
	TmplBlockTime	    string  `json:"tmpl_block_time"`
}

func NewTask(f string) *Task {
	c, err := ioutil.ReadFile(f)
	if err != nil {
		dlog.Warn("fail to read file %s", f)
		return nil
	}
	var task Task
	err = json.Unmarshal(c, &task)
	if err != nil {
		dlog.Warn("fail to get task %s: %s", err.Error(), f)
		return nil
	}
	return &task
}

func (p *Task) DeepCopy() *Task {
	b, _ := json.Marshal(p)
	var ret Task
	json.Unmarshal(b, &ret)
	return &ret
}
