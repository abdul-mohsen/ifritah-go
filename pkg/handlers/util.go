package handlers

import "reflect"

func getCol(item interface{}) []interface{} {

    a := reflect.ValueOf(&item).Elem().Elem()
    numCols := a.NumField()
    columns := make([]interface{}, numCols)
    for i := 0; i < numCols; i++ {
      field := a.Field(i)
      columns[i] = field.Addr().Interface()
    }
  return columns

}

