package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/maerlyn/messenger/fb"
)

var (
	config struct {
		Facebook struct {
			Cookie    string `toml:"cookie"`
			UserAgent string `toml:"user_agent"`
			UserID    string `toml:"user_id"`
		} `toml:"facebook"`
	}

	log *Logger
)

func init() {
	log = NewLogger("log")

	loadConfig()
}

func main() {
	log.App("app starting")

	fbc := fb.NewClient(log, fb.Config{
		Cookie:    config.Facebook.Cookie,
		UserAgent: config.Facebook.UserAgent,
		UserID:    config.Facebook.UserID,
	})

	fbc.Listen()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
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
