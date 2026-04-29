package service

import (
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
	"github.com/ca0fgh/hermestoken/setting/system_setting"
)

func GetCallbackAddress() string {
	if operation_setting.CustomCallbackAddress == "" {
		return system_setting.ServerAddress
	}
	return operation_setting.CustomCallbackAddress
}
