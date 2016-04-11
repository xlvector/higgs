package task

import (
	"encoding/json"
	"github.com/xlvector/dlog"
	"github.com/xlvector/higgs/config"
	"io/ioutil"
	"strings"
)

type TaskManager struct {
	tasks   map[string]*Task
	rootDir string
}

func NewTaskManager(root string) *TaskManager {
	dir, err := ioutil.ReadDir(root)
	if err != nil {
		dlog.Warn("can not find etc folder")
	}
	ret := &TaskManager{
		tasks:   make(map[string]*Task),
		rootDir: root,
	}
	for _, f := range dir {
		if strings.HasSuffix(f.Name(), ".json") {
			task := NewTask("./etc/tmpls/" + f.Name())
			ret.tasks[f.Name()] = task
		}
	}
	ret.FixInclude()
	return ret
}

func (p *TaskManager) Get(tmpl string) *Task {
	if name, ok := config.Instance.Templates[tmpl]; ok {
		dlog.Info("find name %s for tmpl %s", name, tmpl)
		if task, ok2 := p.tasks[name]; ok2 {
			return task.DeepCopy()
		}
	} else {
		dlog.Warn("fail to find name for tmpl %s", tmpl)
	}
	return nil
}

func (p *TaskManager) GetByName(name string) *Task {
	if task, ok := p.tasks[name]; ok {
		return task.DeepCopy()
	}
	return nil
}

func (p *TaskManager) GetJsonByName(name string) string {
	task := p.GetByName(name)
	if task != nil {
		data, _ := json.Marshal(task)
		return string(data)
	}
	return ""
}

func (p *TaskManager) GetJsonByTmpl(tmpl string) string {
	task := p.Get(tmpl)
	if task != nil {
		data, _ := json.Marshal(task)
		return string(data)
	}
	return ""
}

func (p *TaskManager) FixInclude() {
	for k, task := range p.tasks {
		steps := []*Step{}
		hasRequire := false
		for _, step := range task.Steps {
			if step.Require != nil {
				reqTask, ok := p.tasks[step.Require.File]
				if ok {
					dlog.Info("task %s require %s", k, step.Require.File)
					steps = append(steps, p.GetSteps(reqTask, step.Require.From, step.Require.To)...)
				}
				hasRequire = true
			} else {
				steps = append(steps, step)
			}
		}
		if hasRequire {
			task.Steps = steps
		}
		dlog.Info("task %s has %d steps", k, len(task.Steps))
	}
}

func (p *TaskManager) GetSteps(task *Task, start, end string) []*Step {
	ret := []*Step{}
	begin := false
	for _, step := range task.Steps {
		if step.Page == start {
			begin = true
		}
		if begin {
			ret = append(ret, step)
		}
		if len(end) > 0 && step.Page == end {
			break
		}
	}
	return ret
}
