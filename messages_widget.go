package main

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"github.com/maerlyn/messenger/fb"
	"strings"
	"time"
)

type MessagesWidget struct {
	gocui.View

	self *MessagesWidget
	gui  *gocui.Gui
	fbc  *fb.Client

	selectedFriendId string
	messages         map[string][]interface{}

	incomingChannel <-chan interface{}
	outgoingChannel chan<- interface{}
}

const SEPARATOR = "separator"

func NewMessagesWidget(client *fb.Client, g *gocui.Gui, incoming <-chan interface{}, outgoing chan<- interface{}, fica []string) *MessagesWidget {
	w := MessagesWidget{
		fbc:              client,
		gui:              g,
		incomingChannel:  incoming,
		outgoingChannel:  outgoing,
		selectedFriendId: fica[0],
		messages:         make(map[string][]interface{}, 0),
	}
	w.self = &w

	go w.loadLastMessages(fica)

	go w.listenForEvents()

	return &w
}

func (w MessagesWidget) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("messages", 30, 0, maxX-1, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Frame = true
		v.Title = "messages"
		v.Autoscroll = true
		v.Wrap = true

		if err := g.SetKeybinding("messages", gocui.KeyEnter, gocui.ModNone, w.self.insertTerminator); err != nil {
			return err
		}
	}

	return nil
}

func (w *MessagesWidget) insertTerminator(g *gocui.Gui, v *gocui.View) error {
	w.ensureConversation(w.selectedFriendId)

	w.messages[w.selectedFriendId] = append(w.messages[w.selectedFriendId], SEPARATOR)

	w.gui.Update(func(g *gocui.Gui) error {
		v, err := g.View("messages")
		if err != nil {
			panic(err)
		}

		width, _ := w.Size()
		_, _ = fmt.Fprintln(v, strings.Repeat("-", width))

		return nil
	})

	return nil
}

func (w *MessagesWidget) ensureConversation(userId string) {
	if _, ok := w.messages[userId]; !ok {
		w.messages[userId] = make([]interface{}, 0)
	}
}

func (w *MessagesWidget) loadLastMessages(fica []string) {
	time.Sleep(5 * time.Second)

	for _, userId := range fica {
		messages := w.fbc.GetLastMessages(userId, 20)

		for _, m := range messages {
			w.messages[userId] = append(w.messages[userId], m)
		}
		w.messages[userId] = append(w.messages[userId], SEPARATOR)

		if userId == w.selectedFriendId {
			w.updateConversation()
		}

		time.Sleep(5 * time.Second) // to prevent flooding requests to fb
	}
}

func (w *MessagesWidget) updateConversation() {
	w.gui.Update(func(g *gocui.Gui) error {
		v, err := g.View("messages")
		if err != nil {
			return err
		}

		v.Clear()
		_ = v.SetOrigin(0, 0)

		for _, message := range w.messages[w.selectedFriendId] {
			switch msg := message.(type) {
			case fb.Message:
				_, _ = fmt.Fprintf(v, "%s\n", msg.String(fbc))
				//TODO reply
				//TODO reaction

			case string:
				if msg == SEPARATOR {
					widget, err := w.gui.View("messages")
					if err == nil {
						width, _ := widget.Size()
						_, _ = fmt.Fprintf(v, "%s\n", strings.Repeat("-", width))
					}
				}
			}
		}

		return nil
	})
}

func (w *MessagesWidget) listenForEvents() {
	for {
		event := <-w.incomingChannel

		switch event.(type) {
		case SelectedFriendChanged:
			sfc := event.(SelectedFriendChanged)
			w.selectedFriendId = sfc.NewId
			w.updateConversation()

		case fb.Message:
			msg := event.(fb.Message)
			w.ensureConversation(msg.ActorFbId)
			w.messages[msg.ActorFbId] = append(w.messages[msg.ActorFbId], msg)

			if msg.ActorFbId == w.selectedFriendId {
				w.gui.Update(func(g *gocui.Gui) error {
					v, err := g.View("messages")
					if err != nil {
						panic(err)
					}

					_, _ = fmt.Fprintln(v, msg.String(fbc))

					return nil
				})
			}
		}
		//TODO react, reply, stb.
	}
}
