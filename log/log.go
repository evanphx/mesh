package log

import "log"

var Debug = false

func Debugf(str string, args ...interface{}) {
	if !Debug {
		return
	}

	log.Printf("[DEBUG] "+str, args...)
}

func Printf(str string, args ...interface{}) {
	log.Printf(str, args...)
}
