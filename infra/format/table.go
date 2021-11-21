package format

import (
	"fmt"
	"reflect"
	"time"
)

// NewTableFormatter returns formatter converting slice into table string
func NewTableFormatter() Formatter {
	return &tableFormatter{}
}

type tableFormatter struct {
}

// Format formats slice into table string
func (f *tableFormatter) Format(slice interface{}) string {
	sliceValue := reflect.ValueOf(slice)
	sliceType := sliceValue.Type()
	elementType := sliceType.Elem()

	fields := make([]reflect.StructField, 0, elementType.NumField())
	for i := 0; i < elementType.NumField(); i++ {
		field := elementType.Field(i)
		if field.Anonymous {
			continue
		}
		fields = append(fields, field)
	}

	lens := make([]int, len(fields))

	header := make([]string, 0, len(fields))
	for i, field := range fields {
		header = append(header, field.Name)
		if len(field.Name) > lens[i] {
			lens[i] = len(field.Name)
		}
	}
	table := [][]string{header}
	for i := 0; i < sliceValue.Len(); i++ {
		row := make([]string, 0, len(fields))
		elem := sliceValue.Index(i)
		for j, field := range fields {
			field := elem.FieldByName(field.Name)
			value := field.Interface()
			var strValue string
			switch {
			case field.Type() == reflect.TypeOf(time.Time{}):
				strValue = value.(time.Time).Format("2006-01-02 15:04")
			default:
				strValue = fmt.Sprintf("%s", value)
			}
			row = append(row, strValue)
			if len(strValue) > lens[j] {
				lens[j] = len(strValue)
			}
		}
		table = append(table, row)
	}
	res := ""
	for _, row := range table {
		for i, cell := range row {
			res += fmt.Sprintf(fmt.Sprintf(` %%-%ds `, lens[i]), cell)
		}
		res += "\n"
	}

	// remove last new line
	return res[:len(res)-1]
}
