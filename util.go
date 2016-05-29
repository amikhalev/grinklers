package grinklers

import (
	"fmt"
	"reflect"
)

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

func CheckNotNil(ref interface{}, name string) (err error) {
	v := reflect.ValueOf(ref)
	if ref == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		err = fmt.Errorf("%s not specified", name)
	}
	return
}

func CheckRange(ref *int, name string, max int) (err error) {
	if err = CheckNotNil(ref, name); err != nil {
		return
	}
	if *ref < 0 {
		err = fmt.Errorf("%s out of range: %d < 0", name, *ref)
		return
	}
	if *ref >= max {
		err = fmt.Errorf("%s out of range: %d >= %d", name, *ref, max)
		return
	}
	return
}
