package core

import (
	"context"
	"fmt"
	"sync"
)

// ========== 基础Actor框架 ==========
type Message interface{}

type ActorCommandInterface interface {
	Do(actor ActorInterface)
	Response() chan interface{}
}

type ActorCommand struct {
	ActorCommandInterface
	Req interface{}
	Res chan interface{}
}

func (c *ActorCommand) Do(actor ActorInterface) {
	c.ActorCommandInterface.Do(actor)
}

func (c *ActorCommand) Response() chan interface{} {
	return c.Res
}

type ActorInterface interface {
	Send(msg Message)
	Start(owner ActorInterface)
	Stop()
	GetAddress() int
	GetMailbox() chan Message
}

type Actor struct {
	ActorInterface
	address int                //地址
	mailbox chan Message       //消息队列
	ctx     context.Context    //上下文
	cancel  context.CancelFunc //取消函数
	wg      sync.WaitGroup     //等待组
}

func NewActor() *Actor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Actor{
		mailbox: make(chan Message, 100),
		ctx:     ctx,
		cancel:  cancel,
		wg:      sync.WaitGroup{},
	}
}

func (a *Actor) GetAddress() int {
	return a.address
}

func (a *Actor) GetMailbox() chan Message {
	return a.mailbox
}

func (a *Actor) Send(msg Message) {
	select {
	case a.mailbox <- msg:
	case <-a.ctx.Done():
	}
}

func (a *Actor) Start(owner ActorInterface) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case msg := <-a.mailbox:
				msg.(ActorCommandInterface).Do(owner)
			case <-a.ctx.Done():
				fmt.Println("Actor stopped")
				return
			}
		}
	}()
}

func (a *Actor) Stop() {
	a.cancel()
	a.wg.Wait()
	close(a.mailbox)
}
