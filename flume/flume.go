package flume

import (
	"strconv"
	"time"
)

type Flume struct {
	Host string
	Port int
}

func NewFlume(host string, port int) *Flume {
	return &Flume{
		Host: host,
		Port: port,
	}
}

func (p *Flume) Send(bucket string, body []byte) error {
	event := &ThriftFlumeEvent{
		Headers: map[string]string{
			"bucket":    bucket,
			"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
		},
		Body: body,
	}
	c := NewFlumeClient(p.Host, p.Port)
	err := c.Connect()
	if err != nil {
		return err
	}
	defer c.Destroy()
	err = c.Append(event)
	if err != nil {
		return err
	}
	return nil
}
