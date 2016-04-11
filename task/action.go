package task

import (
	"github.com/xlvector/higgs/context"
)

type Action struct {
	Condition     string            `json:"condition"`
	Goto          string            `json:"goto"`
	DeleteContext []string          `json:"delete_context"`
	Message       map[string]string `json:"message"`
	Info          string            `json:"info"`
}

func (p *Action) IsFire(c *context.Context) bool {
	return c.Parse(p.Condition) == "true"
}

func (p *Action) FullInfo(c *context.Context) string {
	return c.Parse(p.Info)
}
