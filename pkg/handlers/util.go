package handlers

import (
	"fmt"
	"reflect"
)

func getCol(item interface{}) []interface{} {

  a := reflect.ValueOf(&item).Elem().Elem()
  fmt.Print(a)

  numCols := a.NumField()
  columns := make([]interface{}, numCols)
  for i := 0; i < numCols; i++ {
    field := a.Field(i)
    columns[i] = field.Addr().Interface()
  }
  return columns

}

