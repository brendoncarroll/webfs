package cells

import (
	"log"
	"reflect"
)

type constructor func(interface{}) Cell

var constructors = map[reflect.Type]constructor{}

func Register(spec interface{}, c func(interface{}) Cell) {
	rtype := reflect.TypeOf(spec)
	constructors[rtype] = c
}

func Make(spec interface{}) Cell {
	rtype := reflect.TypeOf(spec)
	c := constructors[rtype]
	if c == nil {
		log.Println(rtype, constructors)
		panic("no constructor found for spec")
	}
	return c(spec)
}
