module github.com/maerlyn/messenger

go 1.13

replace github.com/maerlyn/messenger/fb => ./fb

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/google/uuid v1.1.1
	github.com/stretchr/testify v1.4.0
	golang.org/x/net v0.0.0-20191126235420-ef20fe5d7933 // indirect
)
