package cmd

import (
	"sync"
	"time"
)

type CommandCache struct {
	data map[string]Command
	lock *sync.RWMutex
}

func NewCommandCache() *CommandCache {
	return &CommandCache{
		data: make(map[string]Command),
		lock: &sync.RWMutex{},
	}
}

func (self *CommandCache) SetCommand(c Command) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.data[c.GetId()] = c
	go func() {
		timer := time.NewTimer(90 * time.Second)
		<-timer.C
		c.Close()
		self.Delete(c.GetId())
	}()
}

func (self *CommandCache) Delete(id string) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if _, ok := self.data[id]; ok {
		delete(self.data, id)
	}
}

func (self *CommandCache) GetCommand(id string) Command {
	self.lock.RLock()
	defer self.lock.RUnlock()

	val, ok := self.data[id]
	if !ok {
		return nil
	}
	return val
}
