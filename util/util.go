package util

import (
	"reflect"
)

// ExhaustChan recieves from a chan until it is closed. c must be a chan
func ExhaustChan(c interface{}) {
	ch := reflect.ValueOf(c)
	if ch.Kind() != reflect.Chan {
		Logger.Panicf("expected channel, received %v", ch.Kind())
	}
	ok := true
	for ok {
		_, ok = ch.TryRecv()
	}
}
