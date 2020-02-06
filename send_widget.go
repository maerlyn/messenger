package main

import (
	"github.com/jroimartin/gocui"
	"github.com/maerlyn/messenger/fb"
)

type SendWidget struct {
	gocui.View

	self *SendWidget
	gui  *gocui.Gui
	fbc  *fb.Client

	selectedFriendId string

	incomingChannel <-chan interface{}
	outgoingChannel chan<- interface{}
}

func NewSendWidget(client *fb.Client, g *gocui.Gui, incoming <-chan interface{}, outgoing chan<- interface{}) *SendWidget {
	r := SendWidget{
		fbc:             client,
		gui:             g,
		incomingChannel: incoming,
		outgoingChannel: outgoing,
	}

	r.self = &r

	r.selectedFriendId = "1792034921"
	go r.listenForEvents()

	return &r
}

func (w SendWidget) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("send", 30, maxY-3, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Frame = true
		v.Title = "send"
		v.Autoscroll = true

		v.Editable = true
		v.Editor = gocui.DefaultEditor

		if err := g.SetKeybinding("send", gocui.KeyEnter, gocui.ModNone, w.self.sendMessage); err != nil {
			return err
		}
		if err := g.SetKeybinding("send", gocui.KeyArrowUp, gocui.ModNone, w.self.cursorUp); err != nil {
			return err
		}
		if err := g.SetKeybinding("send", gocui.KeyArrowDown, gocui.ModNone, w.self.cursorDown); err != nil {
			return err
		}
	}

	return nil
}

func (w *SendWidget) cursorUp(g *gocui.Gui, v *gocui.View) error {
	w.outgoingChannel <- ChangeSelectedFriend{Direction: -1}
	return nil
}

func (w *SendWidget) cursorDown(g *gocui.Gui, v *gocui.View) error {
	w.outgoingChannel <- ChangeSelectedFriend{Direction: +1}
	return nil
}

func (w *SendWidget) listenForEvents() {
	for {
		tmp := <-w.incomingChannel

		switch tmp.(type) {
		case SelectedFriendChanged:
			sfc := tmp.(SelectedFriendChanged)
			w.selectedFriendId = sfc.NewId
		}
	}
}

func (w *SendWidget) sendMessage(g *gocui.Gui, v *gocui.View) error {
	text := v.ViewBuffer()

	w.fbc.SendMessage(w.selectedFriendId, text)

	v.Clear()
	_ = v.SetOrigin(0, 0)
	v.MoveCursor(-len(text), 0, false)

	return nil
}
