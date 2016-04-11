package util

import (
	"testing"
)

func TestSlack(t *testing.T) {
	ret, err := SlackMessage("https://hooks.slack.com/services/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", "#crawler", "higgs", "do unit test")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(ret)
}
