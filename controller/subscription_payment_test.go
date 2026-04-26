package controller

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type subscriptionPaymentAPIResponse struct {
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	URL     string          `json:"url"`
}

func decodeSubscriptionPaymentResponse(t *testing.T, recorderBody []byte) subscriptionPaymentAPIResponse {
	t.Helper()

	var response subscriptionPaymentAPIResponse
	if err := common.Unmarshal(recorderBody, &response); err != nil {
		t.Fatalf("failed to decode subscription payment response: %v", err)
	}
	return response
}

func seedSubscriptionPaymentUser(
	t *testing.T,
	db *gorm.DB,
	userID int,
	email string,
	username string,
	stripeCustomer string,
) *model.User {
	t.Helper()

	user := &model.User{
		Id:             userID,
		Username:       username,
		Password:       "password123",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		Group:          "default",
		Email:          email,
		StripeCustomer: stripeCustomer,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func mustUpdateSubscriptionPlan(t *testing.T, db *gorm.DB, planID int, fields map[string]interface{}) {
	t.Helper()

	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", planID).Updates(fields).Error; err != nil {
		t.Fatalf("failed to update subscription plan: %v", err)
	}
}

func assertFloatEquals(t *testing.T, actual float64, expected float64) {
	t.Helper()

	if math.Abs(actual-expected) > 1e-9 {
		t.Fatalf("expected %.6f, got %.6f", expected, actual)
	}
}

func withSubscriptionStripeSettings(t *testing.T) {
	t.Helper()

	originalSecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalServerAddress := system_setting.ServerAddress

	setting.StripeApiSecret = "sk_test_subscription"
	setting.StripeWebhookSecret = "whsec_subscription"
	system_setting.ServerAddress = "https://example.com"

	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripeWebhookSecret = originalWebhookSecret
		system_setting.ServerAddress = originalServerAddress
	})
}

func withSubscriptionCreemSettings(t *testing.T) {
	t.Helper()

	originalWebhookSecret := setting.CreemWebhookSecret
	originalTestMode := setting.CreemTestMode

	setting.CreemWebhookSecret = "whsec_creem_subscription"
	setting.CreemTestMode = false

	t.Cleanup(func() {
		setting.CreemWebhookSecret = originalWebhookSecret
		setting.CreemTestMode = originalTestMode
	})
}

func withSubscriptionEpaySettings(t *testing.T) {
	t.Helper()

	originalPayMethods := append([]map[string]string(nil), operation_setting.PayMethods...)
	originalServerAddress := system_setting.ServerAddress
	originalPrice := operation_setting.Price

	operation_setting.PayMethods = []map[string]string{
		{
			"name": "支付宝",
			"type": "alipay",
		},
	}
	operation_setting.Price = 6.85
	system_setting.ServerAddress = "https://example.com"

	t.Cleanup(func() {
		operation_setting.PayMethods = originalPayMethods
		operation_setting.Price = originalPrice
		system_setting.ServerAddress = originalServerAddress
	})
}

func TestSubscriptionRequestStripePayPassesQuantityAndStoresAggregateTotal(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "stripe@example.com", "stripe_user", "cus_subscription")
	plan := seedSubscriptionPlan(t, db, "stripe-plan")
	mustUpdateSubscriptionPlan(t, db, plan.Id, map[string]interface{}{
		"price_amount":    30.0,
		"stripe_price_id": "price_subscription",
		"stock_total":     10,
	})
	withSubscriptionStripeSettings(t)

	var captured struct {
		referenceId string
		customerId  string
		email       string
		priceId     string
		quantity    int64
	}

	originalResolver := subscriptionStripeUnitAmountResolver
	subscriptionStripeUnitAmountResolver = func(priceID string) (int64, error) {
		return 3000, nil
	}
	t.Cleanup(func() {
		subscriptionStripeUnitAmountResolver = originalResolver
	})

	originalGenerator := subscriptionStripeCheckoutLinkGenerator
	subscriptionStripeCheckoutLinkGenerator = func(referenceId string, customerId string, email string, priceId string, quantity int64) (string, error) {
		captured.referenceId = referenceId
		captured.customerId = customerId
		captured.email = email
		captured.priceId = priceId
		captured.quantity = quantity
		return "https://stripe.example/checkout", nil
	}
	t.Cleanup(func() {
		subscriptionStripeCheckoutLinkGenerator = originalGenerator
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/stripe/pay", map[string]interface{}{
		"plan_id":  plan.Id,
		"quantity": 3,
	}, user.Id)

	SubscriptionRequestStripePay(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http status 200, got %d", recorder.Code)
	}

	response := decodeSubscriptionPaymentResponse(t, recorder.Body.Bytes())
	if response.Message != "success" {
		t.Fatalf("expected success message, got %s", response.Message)
	}

	var responseData struct {
		PayLink string `json:"pay_link"`
	}
	if err := common.Unmarshal(response.Data, &responseData); err != nil {
		t.Fatalf("failed to decode stripe response data: %v", err)
	}
	if responseData.PayLink != "https://stripe.example/checkout" {
		t.Fatalf("expected checkout link to be returned, got %q", responseData.PayLink)
	}
	if captured.customerId != user.StripeCustomer || captured.email != user.Email || captured.priceId != "price_subscription" {
		t.Fatalf("unexpected stripe checkout args: %+v", captured)
	}
	if captured.quantity != 3 {
		t.Fatalf("expected stripe quantity 3, got %d", captured.quantity)
	}

	var order model.SubscriptionOrder
	if err := db.Where("trade_no = ?", captured.referenceId).First(&order).Error; err != nil {
		t.Fatalf("failed to load created subscription order: %v", err)
	}
	assertFloatEquals(t, order.Money, 90.0)
	if order.Quantity != 3 {
		t.Fatalf("expected order quantity 3, got %d", order.Quantity)
	}
	if order.StockReserved != 3 {
		t.Fatalf("expected reserved stock 3, got %d", order.StockReserved)
	}

	var updatedPlan model.SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&updatedPlan).Error; err != nil {
		t.Fatalf("failed to reload subscription plan: %v", err)
	}
	if updatedPlan.StockLocked != 3 {
		t.Fatalf("expected locked stock 3, got %d", updatedPlan.StockLocked)
	}
}

func TestSubscriptionRequestCreemPayPassesQuantityAndStoresAggregateTotal(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "creem@example.com", "creem_user", "")
	plan := seedSubscriptionPlan(t, db, "creem-plan")
	mustUpdateSubscriptionPlan(t, db, plan.Id, map[string]interface{}{
		"price_amount":     12.5,
		"creem_product_id": "prod_subscription",
		"stock_total":      10,
	})
	withSubscriptionCreemSettings(t)

	var captured struct {
		referenceId string
		productId   string
		email       string
		username    string
		quantity    int
	}

	originalGenerator := subscriptionCreemCheckoutLinkGenerator
	subscriptionCreemCheckoutLinkGenerator = func(referenceId string, product *CreemProduct, email string, username string, quantity int) (string, error) {
		captured.referenceId = referenceId
		captured.productId = product.ProductId
		captured.email = email
		captured.username = username
		captured.quantity = quantity
		return "https://creem.example/checkout", nil
	}
	t.Cleanup(func() {
		subscriptionCreemCheckoutLinkGenerator = originalGenerator
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/creem/pay", map[string]interface{}{
		"plan_id":  plan.Id,
		"quantity": 4,
	}, user.Id)

	SubscriptionRequestCreemPay(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http status 200, got %d", recorder.Code)
	}

	response := decodeSubscriptionPaymentResponse(t, recorder.Body.Bytes())
	if response.Message != "success" {
		t.Fatalf("expected success message, got %s", response.Message)
	}

	var responseData struct {
		CheckoutURL string `json:"checkout_url"`
		OrderID     string `json:"order_id"`
	}
	if err := common.Unmarshal(response.Data, &responseData); err != nil {
		t.Fatalf("failed to decode creem response data: %v", err)
	}
	if responseData.CheckoutURL != "https://creem.example/checkout" {
		t.Fatalf("expected checkout url to be returned, got %q", responseData.CheckoutURL)
	}
	if responseData.OrderID == "" {
		t.Fatal("expected order id to be returned")
	}
	if captured.productId != "prod_subscription" || captured.email != user.Email || captured.username != user.Username {
		t.Fatalf("unexpected creem checkout args: %+v", captured)
	}
	if captured.quantity != 4 {
		t.Fatalf("expected creem quantity 4, got %d", captured.quantity)
	}

	var order model.SubscriptionOrder
	if err := db.Where("trade_no = ?", responseData.OrderID).First(&order).Error; err != nil {
		t.Fatalf("failed to load created subscription order: %v", err)
	}
	assertFloatEquals(t, order.Money, 50.0)
	if order.Quantity != 4 {
		t.Fatalf("expected order quantity 4, got %d", order.Quantity)
	}
	if order.StockReserved != 4 {
		t.Fatalf("expected reserved stock 4, got %d", order.StockReserved)
	}

	var updatedPlan model.SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&updatedPlan).Error; err != nil {
		t.Fatalf("failed to reload subscription plan: %v", err)
	}
	if updatedPlan.StockLocked != 4 {
		t.Fatalf("expected locked stock 4, got %d", updatedPlan.StockLocked)
	}
}

func TestSubscriptionRequestEpayConvertsUsdTotalToGatewayAmount(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	seedSubscriptionPaymentUser(t, db, 1, "epay@example.com", "epay_user", "")
	plan := seedSubscriptionPlan(t, db, "epay-plan")
	mustUpdateSubscriptionPlan(t, db, plan.Id, map[string]interface{}{
		"price_amount": 10.25,
		"stock_total":  10,
	})
	withSubscriptionEpaySettings(t)

	var captured epay.PurchaseArgs

	originalClientProvider := subscriptionEpayClientProvider
	originalPurchase := subscriptionEpayPurchase
	subscriptionEpayClientProvider = func() *epay.Client {
		return &epay.Client{}
	}
	subscriptionEpayPurchase = func(_ *epay.Client, args *epay.PurchaseArgs) (string, map[string]string, error) {
		captured = *args
		return "https://epay.example/pay", map[string]string{"trade_no": args.ServiceTradeNo}, nil
	}
	t.Cleanup(func() {
		subscriptionEpayClientProvider = originalClientProvider
		subscriptionEpayPurchase = originalPurchase
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/epay/pay", map[string]interface{}{
		"plan_id":        plan.Id,
		"quantity":       3,
		"payment_method": "alipay",
	}, 1)

	SubscriptionRequestEpay(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http status 200, got %d", recorder.Code)
	}

	response := decodeSubscriptionPaymentResponse(t, recorder.Body.Bytes())
	if response.Message != "success" {
		t.Fatalf("expected success message, got %s", response.Message)
	}
	if response.URL != "https://epay.example/pay" {
		t.Fatalf("expected epay url to be returned, got %q", response.URL)
	}
	if captured.Type != "alipay" {
		t.Fatalf("expected payment method alipay, got %q", captured.Type)
	}
	if captured.Money != "210.64" {
		t.Fatalf("expected converted gateway money 210.64, got %q", captured.Money)
	}

	var responseData map[string]string
	if err := common.Unmarshal(response.Data, &responseData); err != nil {
		t.Fatalf("failed to decode epay response data: %v", err)
	}
	tradeNo := responseData["trade_no"]
	if tradeNo == "" {
		t.Fatal("expected trade_no in epay response data")
	}

	var order model.SubscriptionOrder
	if err := db.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		t.Fatalf("failed to load created subscription order: %v", err)
	}
	assertFloatEquals(t, order.Money, 30.75)
	assertFloatEquals(t, order.PaymentMoney, 210.64)
	if order.PaymentCurrency != "CNY" {
		t.Fatalf("expected payment currency CNY, got %q", order.PaymentCurrency)
	}
	if order.Quantity != 3 {
		t.Fatalf("expected order quantity 3, got %d", order.Quantity)
	}
	if order.StockReserved != 3 {
		t.Fatalf("expected reserved stock 3, got %d", order.StockReserved)
	}

	var updatedPlan model.SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&updatedPlan).Error; err != nil {
		t.Fatalf("failed to reload subscription plan: %v", err)
	}
	if updatedPlan.StockLocked != 3 {
		t.Fatalf("expected locked stock 3, got %d", updatedPlan.StockLocked)
	}
}

func TestSubscriptionResultURLUsesHistoryQueryForNonFailureStates(t *testing.T) {
	previous := system_setting.ServerAddress
	system_setting.ServerAddress = "https://pay-local.hermestoken.top"
	t.Cleanup(func() {
		system_setting.ServerAddress = previous
	})

	if got := subscriptionResultURL("success"); got != "https://pay-local.hermestoken.top/console/topup?pay=success&show_history=true" {
		t.Fatalf("unexpected success redirect url: %s", got)
	}
	if got := subscriptionResultURL("pending"); got != "https://pay-local.hermestoken.top/console/topup?pay=pending&show_history=true" {
		t.Fatalf("unexpected pending redirect url: %s", got)
	}
	if got := subscriptionResultURL("fail"); got != "https://pay-local.hermestoken.top/console/topup?pay=fail" {
		t.Fatalf("unexpected fail redirect url: %s", got)
	}
}

func TestSubscriptionEpayReturnWithoutParamsRendersBrowserRedirectPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previous := system_setting.ServerAddress
	system_setting.ServerAddress = "https://pay-local.hermestoken.top"
	t.Cleanup(func() {
		system_setting.ServerAddress = previous
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/epay/return", nil)

	SubscriptionEpayReturn(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "window.location.replace(") {
		t.Fatalf("expected browser redirect script, got body: %s", body)
	}
	if !strings.Contains(body, "window.location.replace(\"https://pay-local.hermestoken.top/console/topup?pay=fail\")") {
		t.Fatalf("expected fail redirect target in body, got body: %s", body)
	}
}

func TestSubscriptionEpayReturnSuccessDoesNotRestoreSessionCookie(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "return@example.com", "return_user", "")
	plan := seedSubscriptionPlan(t, db, "return-plan")
	order := &model.SubscriptionOrder{
		UserId:        user.Id,
		PlanId:        plan.Id,
		Money:         plan.PriceAmount,
		TradeNo:       "trade-return-success",
		PaymentMethod: "alipay",
		Status:        common.TopUpStatusPending,
		CreateTime:    1,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to create subscription order: %v", err)
	}

	previousServerAddress := system_setting.ServerAddress
	previousSessionSecret := common.SessionSecret
	system_setting.ServerAddress = "https://pay-local.hermestoken.top"
	common.SessionSecret = "test-session-secret"
	t.Cleanup(func() {
		system_setting.ServerAddress = previousServerAddress
		common.SessionSecret = previousSessionSecret
	})

	originalClientProvider := subscriptionEpayClientProvider
	originalVerify := subscriptionEpayVerify
	subscriptionEpayClientProvider = func() *epay.Client {
		return &epay.Client{}
	}
	subscriptionEpayVerify = func(_ *epay.Client, _ map[string]string) (*epay.VerifyRes, error) {
		return &epay.VerifyRes{
			ServiceTradeNo: order.TradeNo,
			TradeStatus:    epay.StatusTradeSuccess,
			VerifyStatus:   true,
		}, nil
	}
	t.Cleanup(func() {
		subscriptionEpayClientProvider = originalClientProvider
		subscriptionEpayVerify = originalVerify
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	router := gin.New()
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
	router.Use(sessions.Sessions("session", store))
	router.GET("/api/subscription/epay/return", SubscriptionEpayReturn)

	request := httptest.NewRequest(http.MethodGet, "/api/subscription/epay/return?trade_no=ok", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if setCookieHeaders := recorder.Result().Header.Values("Set-Cookie"); len(setCookieHeaders) != 0 {
		t.Fatalf("expected no Set-Cookie header, got headers: %v", setCookieHeaders)
	}
	if !strings.Contains(recorder.Body.String(), "window.location.replace(\"https://pay-local.hermestoken.top/console/topup?pay=success\\u0026show_history=true\")") {
		t.Fatalf("expected success redirect body, got body: %s", recorder.Body.String())
	}
}

func TestSubscriptionEpayNotifyRejectsAmountMismatch(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "subscription-epay-mismatch@example.com", "subscription_epay_mismatch", "")
	plan := seedSubscriptionPlan(t, db, "subscription-epay-mismatch-plan")
	order := &model.SubscriptionOrder{
		UserId:        user.Id,
		PlanId:        plan.Id,
		Money:         99,
		TradeNo:       "trade-epay-amount-mismatch",
		PaymentMethod: "alipay",
		Status:        common.TopUpStatusPending,
		CreateTime:    1,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to create subscription order: %v", err)
	}

	originalClientProvider := subscriptionEpayClientProvider
	subscriptionEpayClientProvider = func() *epay.Client { return &epay.Client{} }
	t.Cleanup(func() {
		subscriptionEpayClientProvider = originalClientProvider
	})

	body := url.Values{}
	body.Set("trade_no", "x")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/epay/notify", strings.NewReader(body.Encode()))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	originalVerify := subscriptionEpayVerify
	subscriptionEpayVerify = func(_ *epay.Client, _ map[string]string) (*epay.VerifyRes, error) {
		return &epay.VerifyRes{
			ServiceTradeNo: order.TradeNo,
			TradeStatus:    epay.StatusTradeSuccess,
			VerifyStatus:   true,
			Type:           "alipay",
			Money:          "0.01",
		}, nil
	}
	t.Cleanup(func() {
		subscriptionEpayVerify = originalVerify
	})

	SubscriptionEpayNotify(ctx)

	if recorder.Body.String() != "fail" {
		t.Fatalf("expected fail body, got %q", recorder.Body.String())
	}

	var reloaded model.SubscriptionOrder
	if err := db.Where("trade_no = ?", order.TradeNo).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload subscription order: %v", err)
	}
	if reloaded.Status != common.TopUpStatusPending {
		t.Fatalf("expected subscription order to remain pending, got %s", reloaded.Status)
	}
}

func TestSubscriptionEpayNotifyAcceptsConvertedGatewayAmount(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "subscription-epay-converted@example.com", "subscription_epay_converted", "")
	plan := seedSubscriptionPlan(t, db, "subscription-epay-converted-plan")
	order := &model.SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           30.75,
		PaymentMoney:    210.64,
		PaymentCurrency: "CNY",
		TradeNo:         "trade-epay-converted",
		PaymentMethod:   "alipay",
		Status:          common.TopUpStatusPending,
		CreateTime:      1,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to create subscription order: %v", err)
	}

	originalClientProvider := subscriptionEpayClientProvider
	subscriptionEpayClientProvider = func() *epay.Client { return &epay.Client{} }
	t.Cleanup(func() {
		subscriptionEpayClientProvider = originalClientProvider
	})

	body := url.Values{}
	body.Set("trade_no", "x")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/epay/notify", strings.NewReader(body.Encode()))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	originalVerify := subscriptionEpayVerify
	subscriptionEpayVerify = func(_ *epay.Client, _ map[string]string) (*epay.VerifyRes, error) {
		return &epay.VerifyRes{
			ServiceTradeNo: order.TradeNo,
			TradeStatus:    epay.StatusTradeSuccess,
			VerifyStatus:   true,
			Type:           "alipay",
			Money:          "210.64",
		}, nil
	}
	t.Cleanup(func() {
		subscriptionEpayVerify = originalVerify
	})

	SubscriptionEpayNotify(ctx)

	if recorder.Body.String() != "success" {
		t.Fatalf("expected success body, got %q", recorder.Body.String())
	}

	var reloaded model.SubscriptionOrder
	if err := db.Where("trade_no = ?", order.TradeNo).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload subscription order: %v", err)
	}
	if reloaded.Status != common.TopUpStatusSuccess {
		t.Fatalf("expected subscription order to be success, got %s", reloaded.Status)
	}
	assertFloatEquals(t, reloaded.Money, 30.75)
	assertFloatEquals(t, reloaded.PaymentMoney, 210.64)

	var topUp model.TopUp
	if err := db.Where("trade_no = ?", order.TradeNo).First(&topUp).Error; err != nil {
		t.Fatalf("failed to load subscription topup row: %v", err)
	}
	assertFloatEquals(t, topUp.Money, 210.64)
	if topUp.Currency != "CNY" {
		t.Fatalf("expected topup currency CNY, got %q", topUp.Currency)
	}
}
