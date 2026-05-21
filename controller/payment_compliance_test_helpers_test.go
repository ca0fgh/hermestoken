package controller

import (
	"testing"

	"github.com/ca0fgh/hermestoken/setting/operation_setting"
)

func confirmPaymentComplianceForTest(t *testing.T) {
	t.Helper()

	paymentSetting := operation_setting.GetPaymentSetting()
	previous := *paymentSetting
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	t.Cleanup(func() {
		*paymentSetting = previous
	})
}
