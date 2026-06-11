package main

import (
	"fmt"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"reflect"
)

func main() {
	t := reflect.TypeOf(&storage.Store{})
	for i := 0; i < t.NumMethod(); i++ {
		fmt.Println(t.Method(i).Name)
	}
}
