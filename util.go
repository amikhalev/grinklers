package main

import (
	"fmt"
	"reflect"
)

func ExhaustChan(c interface{}) {
	ch := reflect.ValueOf(c)
	if ch.Kind() != reflect.Chan {
		panic(fmt.Errorf("expected channel, received %v", ch.Kind()))
	}
	ok := true
	for ok {
		_, ok = ch.TryRecv()
	}
}

func CheckRange(ref *int, name string, max int) (err error) {
	if ref == nil {
		err = fmt.Errorf("%s not specified", name)
		return
	}
	if *ref >= max {
		err = fmt.Errorf("%s out of range: %d >= %d", name, *ref, max)
		return
	}
	return
}
