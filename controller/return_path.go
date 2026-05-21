package controller

import (
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/setting/system_setting"
)

func paymentReturnPath(suffix string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	return base + common.ThemeAwarePath(suffix)
}
