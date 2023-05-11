package steps

import (
	"log"
	"math/rand"
	"time"

	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
)

func fetchPaymentToken(t *shopifyTasks.ShopifyTask) {
	if !t.IsRunning() {
		return
	}
	headers := []map[string]string{
		{"User-Agent": utils.GetUserAgent()},
		{"Content-Type": "application/json"},
		{"Accept": "application/json"},
	}
	//month: 01
	//year: 23
	//cvc: 123
	data := map[string]interface{}{
		"credit_card": map[string]string{
			"month":              t.Profile.Payment.ExpiryMonth,
			"name":               t.Profile.Payment.CardHolderName,
			"number":             t.Profile.Payment.CardNumber,
			"year":               t.Profile.Payment.ExpiryYear,
			"verification_value": t.Profile.Payment.Cvc,
		},
	}
	//TODO: OR https://deposit.us.shopifycs.com/sessions
	resp, err := t.Session.Post("https://elb.deposit.shopifycs.com/sessions", headers, data)
	if err != nil {
		log.Println("[SHOPIFY] error with request (fetchPaymentToken) (will retry):", err)
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		time.Sleep(t.Settings.Shopify.ErrorDelay)

		fetchPaymentToken(t)
	}
	var respStruct paymentTokenResp
	err = resp.Json(&respStruct)
	if err != nil {
		log.Println("[SHOPIFY] error unmarshaling json response (fetchPaymentToken):", err)
		log.Println("[SHOPIFY] Rotating Proxy and retrying")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		time.Sleep(t.Settings.Shopify.ErrorDelay)
		fetchPaymentToken(t)
	}
	if respStruct.Id != "" {
		t.PaymentToken = respStruct.Id
	} else {
		log.Println("[SHOPIFY] error with response (fetchPaymentToken zero value):", err)
		log.Println("[SHOPIFY] Rotating Proxy and retrying")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		time.Sleep(t.Settings.Shopify.ErrorDelay)
		fetchPaymentToken(t)
	}

}

type paymentTokenResp struct {
	Id string `json:"id"`
}
