package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/maerlyn/messenger/fb"
)

type FriendListWidget struct {
	gocui.View

	self *FriendListWidget
	gui  *gocui.Gui
	fbc  *fb.Client

	selectedIndex int

	friendsICareAbout []string
	hasUnread         map[string]bool

	incomingChannel <-chan interface{}
	outgoingChannel chan<- interface{}

	friendsToLoad chan string
}

func NewFriendListWidget(client *fb.Client, g *gocui.Gui, fica []string, incoming <-chan interface{}, outgoing chan<- interface{}) *FriendListWidget {
	w := FriendListWidget{
		fbc:               client,
		gui:               g,
		hasUnread:         make(map[string]bool),
		friendsICareAbout: fica,
		incomingChannel:   incoming,
		outgoingChannel:   outgoing,
		friendsToLoad:     make(chan string, 20),
	}

	w.self = &w

	w.updateFriendsList()
	w.startUpdateLastActiveTimes()
	w.startFriendLoad()

	go w.listenForEvents()

	outgoing <- fica[0]

	return &w
}

func (w *FriendListWidget) changeSelectedFriend(diff int) {
	w.selectedIndex = w.selectedIndex + diff

	if w.selectedIndex < 0 {
		w.selectedIndex = len(w.friendsICareAbout) - 1
	} else if w.selectedIndex >= len(w.friendsICareAbout) {
		w.selectedIndex = 0
	}

	event := SelectedFriendChanged{NewId: w.friendsICareAbout[w.selectedIndex]}
	w.outgoingChannel <- event

	w.updateFriendsList()
}

func (w *FriendListWidget) cursorDown(g *gocui.Gui, v *gocui.View) error {
	w.changeSelectedFriend(+1)

	return nil
}

func (w *FriendListWidget) cursorUp(g *gocui.Gui, v *gocui.View) error {
	w.changeSelectedFriend(-1)

	return nil
}

func (w *FriendListWidget) closeCurrentlySelectedFriend(g *gocui.Gui, v *gocui.View) error {
	var a []string
	for k, v := range w.friendsICareAbout {
		if k != w.selectedIndex {
			a = append(a, v)
		}
	}

	w.friendsICareAbout = a
	w.selectedIndex = 0
	w.changeSelectedFriend(0)

	w.updateFriendsList()

	return nil
}

func (w *FriendListWidget) updateFriendsList() {
	w.gui.Update(func(g *gocui.Gui) error {
		v, err := g.View("friends")
		if err != nil {
			return err
		}

		names := w.fbc.FriendNames()

		v.Clear()
		_, _ = fmt.Fprintln(v, " ")

		for index, id := range w.friendsICareAbout {
			name := names[id]
			if name == "" {
				name = id
				w.friendsToLoad <- id
			}

			lat := w.fbc.LastActive(id)

			if index == w.selectedIndex {
				w.hasUnread[id] = false
			}

			// red dot if unread
			if w.hasUnread[id] {
				_, _ = fmt.Fprint(v, "\033[31m●\033[0m")
			} else {
				_, _ = fmt.Fprint(v, " ")
			}

			if w.isLessThanMinutesAgo(lat, 1) {
				// green dot if recently active
				_, _ = fmt.Fprint(v, " \033[32m●\033[0m ")
			} else if w.isLessThanMinutesAgo(lat, 5) {
				// grey dot if not that recently active
				_, _ = fmt.Fprint(v, " ● ")
			} else {
				// empty dot if not recently active
				_, _ = fmt.Fprint(v, " \033[0m○\033[0m ")
			}

			if w.fbc.IsGroup(id) {
				name = w.fbc.GroupName(id)
			}

			if index == w.selectedIndex {
				// highlight if selected
				_, _ = fmt.Fprintf(v, "\033[37;44m%s\033[0m", name)
			} else {
				_, _ = fmt.Fprintf(v, name)
			}
			_, _ = fmt.Fprintln(v, "")

			if lat > 0 {
				_, _ = fmt.Fprintf(v, "     %s\n", time.Unix(lat, 0).Format("2006-01-02 15:04:05"))
			} else {
				_, _ = fmt.Fprintln(v, "")
			}

			_, _ = fmt.Fprintln(v, "")
		}

		return nil
	})
}

func (w *FriendListWidget) isLessThanMinutesAgo(lat int64, minutes int64) bool {
	return time.Now().Unix()-60*minutes < lat
}

func (w *FriendListWidget) listenForEvents() {
	for {
		tmp := <-w.incomingChannel

		switch obj := tmp.(type) {
		case ChangeSelectedFriend:
			w.changeSelectedFriend(obj.Direction)

		case fb.Message:
			w.markUserUnread(obj.Thread.UniqueId())

		case fb.ReadReceipt:
			w.markUserUnread(obj.Thread.UniqueId())

		case fb.Typing:
			w.markUserUnread(obj.SenderFbId)

		case fb.MarkRead:
			w.hasUnread[obj.Thread.UniqueId()] = false
		}
	}
}

func (w *FriendListWidget) ensureUserInList(userId string) {
	for _, v := range w.friendsICareAbout {
		if userId == v {
			return
		}
	}

	w.friendsICareAbout = append(w.friendsICareAbout, userId)
	w.friendsToLoad <- userId
}

func (w *FriendListWidget) markUserUnread(userId string) {
	w.ensureUserInList(userId)
	w.hasUnread[userId] = true
	w.updateFriendsList()
}

func (w FriendListWidget) Layout(g *gocui.Gui) error {
	_, maxY := g.Size()

	if v, err := g.SetView("friends", 0, 0, 30, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		if err := g.SetKeybinding("friends", gocui.KeyArrowDown, gocui.ModNone, w.self.cursorDown); err != nil {
			return err
		}

		if err := g.SetKeybinding("friends", gocui.KeyArrowUp, gocui.ModNone, w.self.cursorUp); err != nil {
			return err
		}

		if err := g.SetKeybinding("friends", 'c', gocui.ModNone, w.self.closeCurrentlySelectedFriend); err != nil {
			return err
		}

		w.Frame = true
		v.Title = "FRIENDS"

		_, _ = g.SetCurrentView("friends")
	}

	return nil
}

func (w *FriendListWidget) startUpdateLastActiveTimes() {
	ch := make(chan bool)
	w.fbc.RequestNotifyLastActiveTimesUpdate(ch)

	secondsSinceLastUpdate := 0

	go func() {
		t := time.NewTicker(1 * time.Second)

		for {
			<-t.C

			secondsSinceLastUpdate++

			if secondsSinceLastUpdate > 125 {
				os.Exit(1)
			}

			w.gui.Update(func(g *gocui.Gui) error {
				v, _ := w.gui.View("friends")
				v.Title = fmt.Sprintf("%s %d", v.Title[0:len("friends")], secondsSinceLastUpdate)
				return nil
			})
		}
	}()

	go func() {
		for {
			<-ch
			secondsSinceLastUpdate = 0
			w.updateFriendsList()
		}
	}()
}

func (w *FriendListWidget) startFriendLoad() {
	go func() {
		for {
			id := <-w.friendsToLoad
			if w.fbc.LoadFriend(id) {
				w.updateFriendsList()
			}
		}
	}()
}
