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
	log *Logger

	selectedFriendId string
	messages         map[string][]interface{}

	incomingChannel <-chan interface{}
	outgoingChannel chan<- interface{}
}

const SEPARATOR = "separator"

func NewMessagesWidget(client *fb.Client, g *gocui.Gui, incoming <-chan interface{}, outgoing chan<- interface{}, fica []string, log *Logger) *MessagesWidget {
	w := MessagesWidget{
		fbc:              client,
		gui:              g,
		incomingChannel:  incoming,
		outgoingChannel:  outgoing,
		selectedFriendId: fica[0],
		messages:         make(map[string][]interface{}, 0),
		log: log,
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
		if width < 0 {
			width = 1
		}
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

		time.Sleep(1 * time.Second) // to prevent flooding requests to fb
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

			case fb.ReadReceipt:
				_, _ = fmt.Fprintf(v, "%s\n", msg.String(fbc))

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

		switch obj := event.(type) {
		case SelectedFriendChanged:
			w.selectedFriendId = obj.NewId
			w.updateConversation()

		case fb.Message:
			w.ensureConversation(obj.Thread.UniqueId())
			w.messages[obj.Thread.UniqueId()] = append(w.messages[obj.Thread.UniqueId()], obj)
			w.printIfCurrentconversation(obj.Thread.UniqueId(), obj.String(w.fbc))

		case fb.ReadReceipt:
			if obj.IsGroup() {
				continue
			}

			w.ensureConversation(obj.Thread.UniqueId())
			w.messages[obj.Thread.UniqueId()] = append(w.messages[obj.Thread.UniqueId()], obj)
			w.printIfCurrentconversation(obj.Thread.UniqueId(), obj.String(w.fbc))

		case fb.Typing:
			w.ensureConversation(obj.SenderFbId)
			w.messages[obj.SenderFbId] = append(w.messages[obj.SenderFbId], obj)
			w.printIfCurrentconversation(obj.SenderFbId, obj.String(w.fbc))

		case fb.MessageReply:
			w.ensureConversation(obj.RepliedToMessage.Thread.UniqueId())
			if index := w.findMessage(obj.RepliedToMessage.Thread.UniqueId(), obj.RepliedToMessage.MessageId); index != -1 {
				msg := (w.messages[obj.RepliedToMessage.Thread.UniqueId()][index]).(fb.Message)
				msg.Replies = append(msg.Replies, obj)
				w.messages[obj.RepliedToMessage.Thread.UniqueId()][index] = msg

				if msg.Thread.UniqueId() == w.selectedFriendId {
					w.updateConversation()
				}
			} else {
				log.Error(fmt.Sprintf("[messages] cannot find message %s", obj.RepliedToMessage.MessageId))
			}

		case fb.MessageReaction:
			w.ensureConversation(obj.Thread.UniqueId())
			if index := w.findMessage(obj.Thread.UniqueId(), obj.MessageId); index != -1 {
				msg := (w.messages[obj.Thread.UniqueId()][index]).(fb.Message)
				msg.Reactions = append(msg.Reactions, obj)
				w.messages[obj.Thread.UniqueId()][index] = msg

				if msg.Thread.UniqueId() == w.selectedFriendId {
					w.updateConversation()
				}
			} else {
				log.Error(fmt.Sprintf("[messages] cannot find message %s", obj.MessageId))
			}
		}
	}
}

func (w *MessagesWidget) printIfCurrentconversation(userId, text string) {
	if w.selectedFriendId != userId {
		return
	}

	w.gui.Update(func(g *gocui.Gui) error {
		v, err := g.View("messages")
		if err != nil {
			panic(err)
		}

		_, _ = fmt.Fprintln(v, text)

		return nil
	})
}

func (w *MessagesWidget) findMessage(uniqueId, messageId string) int {
	for index, obj := range w.messages[uniqueId] {
		if msg, ok := obj.(fb.Message); ok {
			if msg.MessageId == messageId {
				return index
			}
		}
	}

	return -1
}
