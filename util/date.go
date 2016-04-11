package util

import (
	"github.com/xlvector/dlog"
	"time"
)

func DateFormatTransfer(tm, srcFmt, dstFmt string) (string, error) {
	t, err := time.Parse(srcFmt, tm)
	if err != nil {
		dlog.Warn("parse date failed: %s", err.Error())
		return "", err
	}
	return t.Format(dstFmt), nil
}
