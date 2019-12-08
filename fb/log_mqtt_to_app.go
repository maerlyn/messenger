package fb

import "fmt"

type logMQTTToApp struct {
	appLogger Logger
}

func (l logMQTTToApp) Println(v ...interface{}) {
	l.appLogger.Error(fmt.Sprintln(v...))
}

func (l logMQTTToApp) Printf(format string, v ...interface{}) {
	l.appLogger.Error(fmt.Sprintf(format, v...))
}
