package util

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func SlackMessage(api, channel, user, text string) (string, error) {
	data := map[string]interface{}{
		"text":     text,
		"username": user,
		"channel":  channel,
	}
	b, _ := json.Marshal(data)
	log.Println(string(b))
	resp, err := http.Post(api, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
