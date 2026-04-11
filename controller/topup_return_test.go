package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func TestTopUpResultURL(t *testing.T) {
	previous := system_setting.ServerAddress
	system_setting.ServerAddress = "https://pay-local.hermestoken.top"
	t.Cleanup(func() {
		system_setting.ServerAddress = previous
	})

	if got := topUpResultURL("success"); got != "https://pay-local.hermestoken.top/console/topup?pay=success&show_history=true" {
		t.Fatalf("unexpected success redirect url: %s", got)
	}
	if got := topUpResultURL("pending"); got != "https://pay-local.hermestoken.top/console/topup?pay=pending&show_history=true" {
		t.Fatalf("unexpected pending redirect url: %s", got)
	}
	if got := topUpResultURL("fail"); got != "https://pay-local.hermestoken.top/console/topup?pay=fail" {
		t.Fatalf("unexpected fail redirect url: %s", got)
	}
}

func TestRenderBrowserRedirectWritesHTMLPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	target := "https://pay-local.hermestoken.top/console/topup?pay=success&show_history=true"
	renderBrowserRedirect(ctx, target)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "window.location.replace(") {
		t.Fatalf("expected browser redirect script, got body: %s", body)
	}
	if !strings.Contains(body, "window.location.replace(\"https://pay-local.hermestoken.top/console/topup?pay=success\\u0026show_history=true\")") {
		t.Fatalf("expected target url in body, got body: %s", body)
	}
}

func TestParseEpayParamsSupportsGetAndPost(t *testing.T) {
	gin.SetMode(gin.TestMode)

	getRecorder := httptest.NewRecorder()
	getContext, _ := gin.CreateTestContext(getRecorder)
	getContext.Request = httptest.NewRequest(http.MethodGet, "/api/user/epay/return?trade_no=abc123&status=ok", nil)
	getParams, err := parseEpayParams(getContext)
	if err != nil {
		t.Fatalf("expected GET params parse to succeed, got error: %v", err)
	}
	if getParams["trade_no"] != "abc123" || getParams["status"] != "ok" {
		t.Fatalf("unexpected GET params: %+v", getParams)
	}

	postRecorder := httptest.NewRecorder()
	postContext, _ := gin.CreateTestContext(postRecorder)
	form := url.Values{}
	form.Set("trade_no", "xyz789")
	form.Set("status", "ok")
	postContext.Request = httptest.NewRequest(http.MethodPost, "/api/user/epay/return", strings.NewReader(form.Encode()))
	postContext.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postParams, err := parseEpayParams(postContext)
	if err != nil {
		t.Fatalf("expected POST params parse to succeed, got error: %v", err)
	}
	if postParams["trade_no"] != "xyz789" || postParams["status"] != "ok" {
		t.Fatalf("unexpected POST params: %+v", postParams)
	}
}
