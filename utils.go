package mobile_service

import (
	"fmt"
	"strings"
)

func AddTrailingSlash(initial string) string {
	if strings.HasSuffix(initial, "/") {
		return initial
	}
	return fmt.Sprintf("%s/", initial)
}
