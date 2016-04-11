package util

import (
	"testing"
)

func TestDateFormatTransfer(t *testing.T) {
	tm, err := DateFormatTransfer("2015-12-13 13:54:09", "2006-01-02 15:04:05", "01/02/2006 15:04")
	if err != nil {
		t.Error(err)
	}
	if tm != "12/13/2015 13:54" {
		t.Error(tm)
	}
}
