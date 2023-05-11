package steps

import (
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
)

func FetchPaymentGateway(t *shopifyTasks.ShopifyTask) error {
	headers := []map[string]string{
		{"user-agent": utils.GetUserAgent()},
		{"content-type": "application/x-www-form-urlencoded"},
	}
	var (
		err  error
		iter int
	)
	for !strings.Contains(t.LastResponse.Text, `data-gateway-name="credit_card"`) {
		if !t.IsRunning() {
			return errs.ErrorTaskStop
		}
		t.LastResponse, err = t.Session.Get(t.SessionUrl+"?previous_step=shipping_method&step=payment_method", headers, nil)
		if err != nil {
			log.Println("[SHOPIFY] error with request (fetchPaymentToken):", err)
			log.Println("[SHOPIFY] Rotating Proxy and retrying")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			t.Session.SetProxy(newProxy)
			return errs.ErrorWithRequest
		}
		if iter == 0 {
			log.Println("[SHOPIFY] Awaiting Payment Gateway:", iter)
			t.SetStatus("Awaiting Payment Gateway", colors.Yellow)
		} else if iter > 1 {
			log.Println("[SHOPIFY] Awaiting Restock", iter)
			t.SetStatus("Awaiting Restock", colors.Yellow)
			time.Sleep(t.Settings.Supreme.MonitorDelay) //Waiting for restock
		}
		iter++
	}
	if !strings.Contains(t.LastResponse.Text, `data-gateway-name="credit_card"`) && strings.Contains(t.LastResponse.Text, `'data-select-gateway='`) {
		log.Println("[SHOPIFY] CREDIT CARD NOT ACCEPTED - EXITING!")
		return errs.ErrorFatalStopTask
	}
	parseAuthTokenAndSiteKey(t.LastResponse.Text, t)
	totalPrice := strings.TrimSpace(strings.Split(strings.Split(t.LastResponse.Text, `data-checkout-payment-due-target="`)[1], `"`)[0])
	t.PaymentGateway = strings.TrimSpace(strings.Split(strings.Split(strings.Split(t.LastResponse.Text, `data-gateway-name="credit_card"`)[1], `data-select-gateway="`)[1], `"`)[0])
	if len(totalPrice) > 2 {
		log.Printf("Checkout Total: %v.%v\n", totalPrice[:len(totalPrice)-2], totalPrice[len(totalPrice)-2:])
	} else {
		log.Printf("Checkout Total: %v", totalPrice)
	}
	t.CheckoutTotal = totalPrice
	if t.PaymentToken == "" {
		log.Println("[SHOPIFY] Awaiting Payment Token")
	}
	for {
		if t.PaymentToken != "" {
			return nil
		}
	}

}
