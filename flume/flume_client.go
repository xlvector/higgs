package flume

import (
	"errors"
	"fmt"
	"github.com/apache/thrift/lib/go/thrift"
	"github.com/xlvector/dlog"
	"net"
	"strconv"
	"time"
)

const (
	STATUS_INIT  int32 = 0
	STATUS_READY int32 = 1
	STATUS_DEAD  int32 = 2
)

type FlumeClient struct {
	host             string
	port             int
	tsocket          *thrift.TSocket
	transport        thrift.TTransport
	transportFactory thrift.TTransportFactory
	protocolFactory  *thrift.TCompactProtocolFactory
	thriftclient     *ThriftSourceProtocolClient
	status           int32 //连接状态
}

func NewFlumeClient(host string, port int) *FlumeClient {

	return &FlumeClient{host: host, port: port, status: STATUS_INIT}

}

func (self *FlumeClient) IsAlive() bool {
	return self.status == STATUS_READY

}

func (self *FlumeClient) Connect() error {

	var tsocket *thrift.TSocket
	var err error
	//创建一个物理连接
	tsocket, err = thrift.NewTSocketTimeout(net.JoinHostPort(self.host, strconv.Itoa(self.port)), 10*time.Second)
	if nil != err {
		dlog.Warn("FLUME_CLIENT|CREATE TSOCKET|FAIL|%s|%s", self.HostPort(), err)
		return err
	}

	self.tsocket = tsocket
	self.transportFactory = thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	//TLV 方式传输
	self.protocolFactory = thrift.NewTCompactProtocolFactory()

	self.clientConn()
	self.status = STATUS_READY
	//go self.checkAlive()

	return nil
}

func (self *FlumeClient) clientConn() error {
	//使用非阻塞io来传输
	self.transport = self.transportFactory.GetTransport(self.tsocket)
	self.thriftclient = NewThriftSourceProtocolClientFactory(self.transport, self.protocolFactory)
	if err := self.transport.Open(); nil != err {
		dlog.Warn("FLUME_CLIENT|CREATE THRIFT CLIENT|FAIL|%s|%s", self.HostPort(), err)
		return err
	}
	return nil
}

func (self *FlumeClient) checkAlive() {
	for self.status != STATUS_DEAD {
		//休息1s
		time.Sleep(1 * time.Second)
		isOpen := self.tsocket.IsOpen()
		if !isOpen {
			self.status = STATUS_DEAD
			dlog.Warn("flume : %s:%d is Dead", self.host, self.port)
			break
		}

	}
}

func (self *FlumeClient) AppendBatch(events []*ThriftFlumeEvent) error {
	return self.innerSend(func() (Status, error) {
		return self.thriftclient.AppendBatch(events)
	})
}

func (self *FlumeClient) Append(event *ThriftFlumeEvent) error {

	return self.innerSend(func() (Status, error) {
		return self.thriftclient.Append(event)
	})
}

func (self *FlumeClient) innerSend(sendfunc func() (Status, error)) error {

	if self.status == STATUS_DEAD {
		return errors.New("FLUME_CLIENT|DEAD|" + self.HostPort())
	}

	//如果transport关闭了那么久重新打开
	if !self.transport.IsOpen() {
		//重新建立thriftclient
		err := self.clientConn()
		if nil != err {
			dlog.Warn("FLUME_CLIENT|SEND EVENT|CLIENT CONN|CREATE FAIL|%s|%s", self.HostPort(), err.Error())
			return err
		}
	}

	status, err := sendfunc()
	if nil != err {
		dlog.Warn("FLUME_CLIENT|SEND EVENT|FAIL|%s|%s|STATUS:%s", self.HostPort(), err.Error(), status)
		status = Status_ERROR
		self.status = STATUS_DEAD
	}

	//如果没有成功则向上抛出
	if status != Status_OK {
		return errors.New("deliver fail ! " + self.HostPort() + "|" + status.String())
	}
	return nil
}

func (self *FlumeClient) Destroy() {

	self.status = STATUS_DEAD
	err := self.transport.Close()
	if nil != err {
		dlog.Warn("%v", err.Error())
	}

}

func NewFlumeEvent() ThriftFlumeEvent {
	return *NewThriftFlumeEvent()
}

func EventFillUp(obji interface{}, business, action string, body []byte) *ThriftFlumeEvent {
	event := obji.(ThriftFlumeEvent)
	//拼装头部信息
	header := make(map[string]string, 2)
	header["businessName"] = business
	header["type"] = action
	event.Headers = header
	event.Body = body

	return &event
}

func (self *FlumeClient) HostPort() string {
	return fmt.Sprintf("[%s:%d-%d]", self.host, self.port, self.status)
}
