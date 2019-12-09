package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jroimartin/gocui"
	"github.com/maerlyn/messenger/fb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	config struct {
		Main struct {
			FriendsICareAbout   []string `toml:"friends_i_care_about"`
			EnableSend          bool     `toml:"enable_send"`
			EnableNotifications bool     `toml:"enable_notifications"`
			ActivePing          bool     `toml:"active_ping"`
		}
		Facebook struct {
			Cookie    string `toml:"cookie"`
			UserAgent string `toml:"user_agent"`
			UserID    string `toml:"user_id"`
		} `toml:"facebook"`
	}

	log       *Logger
	fbc       *fb.Client
	nextViews map[string]string
)

func init() {
	log = NewLogger("log")

	loadConfig()

	nextViews = make(map[string]string)

	//TODO
	//nextViews["friends"] = "messages"
	//nextViews["messages"] = "send"
	//nextViews["send"] = "friends"

	nextViews["friends"] = "send"
	nextViews["send"] = "friends"

	if !config.Main.EnableSend {
		delete(nextViews, "send")
		nextViews["messages"] = "friends"
	}

	stderrFile, err := os.OpenFile("stderr", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		panic("failed to redirect stderr to file: " + err.Error())
	}
	err = syscall.Dup2(int(stderrFile.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		panic("failed to redirect stderr to file2: " + err.Error())
	}
	_, _ = fmt.Fprintln(stderrFile, strings.Repeat("-", 40))

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	go func() {
		for {
			<-signals
			log.App("USR1 received, reloading config")

			loadConfig()
		}
	}()
}

func main() {
	log.App("app starting")

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Error(fmt.Sprintf("%s\n", err))
		os.Exit(1)
	}
	defer g.Close()

	fbc = fb.NewClient(log, fb.Config{
		Cookie:    config.Facebook.Cookie,
		UserAgent: config.Facebook.UserAgent,
		UserID:    config.Facebook.UserID,
	})

	//go InitPrometheus(fbc, log)

	//go func() {
	//	//fetch friend images after 5s
	//	t := time.NewTimer(5 * time.Second)
	//	<-t.C
	//
	//	for _, friend := range fbc.FriendList {
	//		DownloadFriendImage(friend)
	//	}
	//}()

	//TODO create channels

	friendListWidgetIn := make(chan interface{}, 10)
	friendListWidgetOut := make(chan interface{}, 10)
	friendListWidget := NewFriendListWidget(fbc, g, config.Main.FriendsICareAbout, friendListWidgetIn, friendListWidgetOut)

	//messagesWidget := NewMessagesWidget(fbc, g, mainLogger, config.Main.FriendsICareAbout, config.EmojisToReplace)
	//sendWidget := NewSendWidget(fbc, g, mainLogger, &friendListWidget)

	sendWidgetIn := make(chan interface{}, 10)
	sendWidgetOut := make(chan interface{}, 10)
	sendWidget := NewSendWidget(fbc, g, sendWidgetIn, sendWidgetOut)

	g.SetManager(
		friendListWidget,
		//messagesWidget,
	)

	if config.Main.EnableSend {
		g.SetManager(
			friendListWidget,
			//messagesWidget,
			sendWidget,
		)
	}

	fbEventsChannel := make(chan interface{}, 10)
	fbc.SetEventChannel(fbEventsChannel)
	fbc.Listen()

	//TODO set & monitor channels
	go func() {
		for {
			select {
			case _ = <-fbEventsChannel:
				//noop (for now)

			case tmp := <-friendListWidgetOut:
				sendWidgetIn <- tmp

			case tmp := <-sendWidgetOut:
				friendListWidgetIn <- tmp
			}
		}
	}()

	go initPrometheus()

	if err := keybindings(g); err != nil {
		log.Error(fmt.Sprintf("%s\n", err))
		os.Exit(1)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Error(fmt.Sprintf("%s\n", err))
		os.Exit(1)
	}
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		_, err := g.SetCurrentView("friends")
		return err
	}

	v.Title = strings.ToLower(v.Title)

	_, err := g.SetCurrentView(nextViews[v.Name()])

	if g.CurrentView().Name() == "send" {
		g.Cursor = true
	} else {
		g.Cursor = false
	}

	if err != nil {
		return err
	}

	g.CurrentView().Title = strings.ToUpper(g.CurrentView().Title)

	return nil
}

func quit(_ *gocui.Gui, _ *gocui.View) error {
	return gocui.ErrQuit
}

func keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}

	return nil
}

func loadConfig() {
	isFirst := config.Facebook.UserAgent == ""

	_, err := toml.DecodeFile("config.toml", &config)

	if isFirst && err != nil {
		panic(fmt.Sprintf("cannot decode config: %+v", err))
	}

	if err != nil {
		log.App("config reload failed")
		log.Error(fmt.Sprintf("cannot decode config: %+v", err))
		return
	}

	log.App(fmt.Sprintf("loaded config %+v", config))
}

func initPrometheus() {
	metrics := make(map[string]prometheus.Gauge)
	timer := time.NewTicker(5 * time.Second)

	if config.Facebook.UserID == "561598959" {
		metrics["1792034921"] = promauto.NewGauge(prometheus.GaugeOpts{
			Name: "fb_user_1792034921",
		})

		metrics["1792034921"].Set(0)
	}

	go func() {
		for {
			for userId, lat := range fbc.LastActiveAll() {
				if _, ok := metrics[userId]; !ok {
					metrics[userId] = promauto.NewGauge(prometheus.GaugeOpts{
						Name: fmt.Sprintf("fb_user_%s", userId),
					})
				}

				if time.Now().Unix()-60 <= lat {
					metrics[userId].Set(1)
				} else {
					metrics[userId].Set(0)
				}
			}

			<-timer.C
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(":2112", nil)
	if err != nil {
		log.Error(fmt.Sprintf("cannot listen on :2112: %s", err))
	}
}
