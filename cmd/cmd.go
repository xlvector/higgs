package cmd

import (
	"crypto/rsa"
	"net/url"
)

type Command interface {
	GetMessage() *Output
	SetInputArgs(map[string]string)
	Finished() bool
	Successed() bool
	GetId() string
	Close() bool
}

type CommandFactory interface {
	CreateCommand(url.Values) Command
	CreateCommandWithPrivateKey(url.Values, *rsa.PrivateKey) Command
}

const (
	PARAM_USERNAME    = "username"
	PARAM_PASSWORD    = "password"
	PARAM_PASSWORD2   = "password2"
	PARAM_VERIFY_CODE = "randcode"
	PARAM_PHONE_NUM   = "phone"

	FAIL                  = "fail"
	NEED_PARAM            = "need_param"
	NOT_SUPPORT           = "not_support"
	WRONG_PASSWORD        = "wrong_password"
	WRONG_VERIFYCODE      = "wrong_verifycode"
	WRONG_SECOND_PASSWORD = "wrong_second_password"
	LOGIN_SUCCESS         = "login_success"
	BEGIN_FETCH_DATA      = "begin_fetch_data"
	FINISH_FETCH_DATA     = "finish_fetch_data"
	FINISH_ALL            = "finish_all"
	OUTPUT_PUBLICKEY      = "output_publickey"
	OUTPUT_VERIFYCODE     = "output_verifycode"
	OUTPUT_QRCODE         = "output_qrcode"
)

type Output struct {
	Status    string `json:"status"`
	NeedParam string `json:"need_param"`
	Id        string `json:"id"`
	Data      string `json:"data"`
}
