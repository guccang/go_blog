package core

import (
	"context"
	"fmt"
	"sync"
)

// ========== 基础Actor框架 ==========
type Message interface{}

type ActorCommand interface {
	Message
	Execute(actor ActorInterface)
}

type ActorInterface interface {
	Send(msg Message)
	Start(behavior ActorBehavior)
	Stop()
	GetAddress() int
	GetMailbox() chan Message
}

type ActorBehavior func(msg ActorCommand, actor ActorInterface)

type Actor struct {
	ActorInterface
	address  int                //地址
	mailbox  chan Message       //消息队列
	ctx      context.Context    //上下文
	cancel   context.CancelFunc //取消函数
	wg       sync.WaitGroup     //等待组
	behavior ActorBehavior      //行为函数
}

func NewActor(address int, bufferSize int) *Actor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Actor{
		address:  address,
		mailbox:  make(chan Message, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
		wg:       sync.WaitGroup{},
		behavior: nil,
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

func (a *Actor) Start(behavior ActorBehavior) {
	a.behavior = behavior
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case msg := <-a.mailbox:
				a.behavior(msg.(ActorCommand), a)
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

// ========== 示例：银行账户Actor ==========
type AccountActor struct {
	*Actor
	balance int
}

type AccountCommand interface {
	Message
	Execute(account *AccountActor)
}

type Deposit struct{ Amount int }
type Withdraw struct{ Amount int }
type GetBalance struct{ Reply chan int }

func (d Deposit) Execute(acc *AccountActor) {
	acc.balance += d.Amount
	fmt.Printf("Deposited %d. New balance: %d\n", d.Amount, acc.balance)
}

func (w Withdraw) Execute(acc *AccountActor) {
	if acc.balance >= w.Amount {
		acc.balance -= w.Amount
		fmt.Printf("Withdrew %d. New balance: %d\n", w.Amount, acc.balance)
	} else {
		fmt.Printf("Insufficient funds for %d. Balance: %d\n", w.Amount, acc.balance)
	}
}

func (g GetBalance) Execute(acc *AccountActor) {
	g.Reply <- acc.balance
}

func NewAccountActor() *AccountActor {
	actor := NewActor(10, 100)
	account := &AccountActor{
		Actor:   actor,
		balance: 0,
	}

	account.Start(func(msg ActorCommand, actor ActorInterface) {
		msg.Execute(account)
	})

	return account
}

// ========== 监督策略 ==========
type Supervisor struct {
	actors []*Actor
}

func (s *Supervisor) Monitor(actor *Actor, restartPolicy bool) {
	s.actors = append(s.actors, actor)
	go func() {
		<-actor.ctx.Done() // 等待actor停止
		if restartPolicy {
			fmt.Println("Restarting failed actor...")
			// 实际项目中应添加重启逻辑
			actor.Start(actor.behavior)
		}
	}()
}

type CmdError struct {
	ActorCommand
	Cmd  string
	Code int
	Msg  string
}

func (e *CmdError) Execute(ractor *Actor) {
	ractor.Send(e)
}
