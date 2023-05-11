package steps

import (
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
	"github.com/adam-0001/requests"
)

func SubmitPayment(t *shopifyTasks.ShopifyTask) error {
	if !t.IsRunning() {
		return errs.ErrorTaskStop
	}
	t.InStock = true
	var (
		cleanedHashes map[string]string
		splitHashes   []string
	)
	splitHashes = strings.Split(t.LastResponse.Text, `<textarea name="`)
	if len(splitHashes) > 1 {
		splitHashes = splitHashes[1:]
		for i := range splitHashes {
			cleanedHashes[strings.Split(splitHashes[i+1], `"`)[0]] = ""
		}
		log.Println("[SHOPIFY] Got hashes (submitPayment):", cleanedHashes)
	}

	payload := url.Values{}
	payload.Set("_method", "patch")
	payload.Set("authenticity_token", t.AuthenticityToken)
	payload.Set("previous_step", "payment_method")
	payload.Set("step", "")
	payload.Set("s", t.PaymentToken)
	payload.Set(t.CheckoutToken+"-count", fmt.Sprint(len(cleanedHashes)))
	payload.Set(t.CheckoutToken+"-count", "fs_count")
	payload.Set("checkout[payment_gateway]", t.PaymentGateway)
	payload.Set("checkout[credit_card][vault]", "false")
	payload.Set("checkout[different_billing_address]", "false")
	payload.Set("checkout[remember_me]", "false")
	payload.Set("checkout[remember_me]", "0")
	payload.Set("complete", "1")
	payload.Set("checkout[client_details][browser_width]", "943")
	payload.Set("checkout[client_details][browser_height]", "969")
	payload.Set("checkout[client_details][javascript_enabled]", "1")
	payload.Set("checkout[client_details][color_depth]", "24")
	payload.Set("checkout[client_details][java_enabled]", "0")
	payload.Set("checkout[client_details][browser_tz]", "240")
	payload.Set("checkout[attributes][I agree to the Terms and Conditions]", "Yes")
	payload.Set("checkout[total_price]", t.CheckoutTotal)
	for _, item := range cleanedHashes {
		payload.Set(item, "")
	}
	headers := []map[string]string{
		{"user-agent": utils.GetUserAgent()},
		{"content-type": "application/x-www-form-urlencoded"},
	}
	log.Println("[SHOPIFY] Submitting payment...")
	t.SetStatus("Submitting payment", colors.Blue)
	for {
		if !t.IsRunning() {
			return errs.ErrorTaskStop
		}
		resp, err := t.Session.Post(t.SessionUrl, headers, payload.Encode())
		if err != nil {
			log.Println("[SHOPIFY] error with request (submitPayment):", err)
			log.Println("[SHOPIFY] Rotating Proxy")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			errs.HandleErrorStatus(t, errs.ErrorWithRequest)
			t.Session.SetProxy(newProxy)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			continue
			// return errs.ErrorWithRequest
		}
		if resp.StatusCode() >= 400 {
			log.Printf("[SHOPIFY] error submitting payment %v- retrying...\n", resp.StatusCode())
			log.Println("[SHOPIFY] Rotating Proxy")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			errs.HandleErrorStatus(t, errs.ErrUnexpectedResponse)
			t.Session.SetProxy(newProxy)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			continue
		}
		//TODO: add handling for checkout captcha
		if strings.Contains(strings.ToLower(resp.Text), "complete the recaptcha to continue") {
			log.Println("[SHOPIFY] CHECKOUT CAPTCHA ENFORCED")
			t.End("Stopped - Checkout Captcha Enforced")
			return errs.ErrorTaskStop
		}
		var suffix = "/processing"
		for loop := 0; loop < 10; loop++ {
			t.SetStatus("Processing Payment", colors.Blue)
			if !t.IsRunning() {
				return errs.ErrorTaskStop
			}
			time.Sleep(time.Millisecond * 600)
			log.Println("[SHOPIFY] Polling Checkout -", loop)
			headers := []map[string]string{
				{"user-agent": utils.GetUserAgent()},
			}
			var resp requests.Response
			var err error
			for {
				resp, err = t.Session.Get(t.SessionUrl+suffix, headers, nil)
				if err != nil {
					log.Println("[SHOPIFY] error with request (polling checkout):", err)
					log.Println("[SHOPIFY] Rotating Proxy")
					newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
					t.Session.SetProxy(newProxy)
					errs.HandleErrorStatus(t, errs.ErrorWithRequest)
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					time.Sleep(t.Settings.Shopify.ErrorDelay)
					continue
				}
				break
			}
			if strings.Contains(resp.Url(), "thank_you") || strings.Contains(resp.Url(), "orders") {
				log.Println("[SHOPIFY] Successfully Checked Out")
				if ex := strings.Split(resp.Text, `class="os-order-number"`); len(ex) > 1 {
					ex1 := strings.Split(resp.Text, `class="os-order-number"`)[1]
					ex2 := strings.TrimSpace(strings.Split(ex1, `</`)[0])
					if ordNum := strings.Split(ex2, "#"); len(ordNum) > 1 {
						t.OrderNumber = ordNum[1]
					}
				}
				if len(resp.RedirectHistory) > 1 {
					for _, redir := range resp.RedirectHistory {
						if strings.Contains(redir, "key=") {
							t.CheckoutUrl = redir
						}
					}
				}
				return nil
			} else {
				if strings.Contains(resp.Text, `<p class="notice__text">`) {
					var reason string = "Card Declined"
					if x := strings.Split(resp.Text, `<p class="notice__text">`); len(x) > 1 {
						reason = strings.TrimSpace(strings.Split(x[1], `</p>`)[0])
					}
					if strings.Contains(strings.ToLower(reason), "gateway") {
						reason += ` (Invalid Card Info/Out of Stock)`
					}
					log.Println("[SHOPIFY] Error submitting payment:", reason)
					t.SetStatus("Card Decline - Retrying", colors.Red)
					t.Session.SetProxy(t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))])
					log.Println("[SHOPIFY] Getting New Payment Token...")
					t.LastResponse = resp
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					fetchPaymentToken(t)
					t.FailedCheckout()
					time.Sleep(t.Settings.Shopify.ErrorDelay)
					return SubmitPayment(t)
				}

			}
			suffix = "/processing?from_processing_page=1"
		}
		log.Println("[SHOPIFY] Error submitting payment - retrying...")
		t.SetStatus("Poll Timeout! - Retrying...", colors.Red)
		time.Sleep(t.Settings.Shopify.ErrorDelay)
	}

}
