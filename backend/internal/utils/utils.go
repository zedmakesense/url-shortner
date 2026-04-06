package utils

import (
	"reflect"
	"regexp"
)

func IsValidEmail(email string) bool {
	var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return false
	}
	return true
}

func IsStructEmpty(s interface{}) bool {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.IsZero() { // Go 1.13+: Checks zero value directly
			return false
		}
	}
	return true
}
