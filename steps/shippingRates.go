package steps

import (
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

func SetShippingRate(t *shopifyTasks.ShopifyTask) error {
	headers := []map[string]string{
		{"user-agent": utils.GetUserAgent()},
	}
	parseAuthTokenAndSiteKey(t.LastResponse.Text, t)
	var (
		chosenRate string
		iter       int
		err        error
	)
	for {
		if !t.IsRunning() {
			return errs.ErrorTaskStop
		}
		t.SetStatus("Getting Shipping Rates", colors.Yellow)
		if strings.Contains(t.LastResponse.Text, "does not require shipping") {
			log.Println("[SHOPIFY] No shipping required")
			break
		}
		if strings.Contains(t.LastResponse.Text, "checkout[shipping_rate][id]") {
			chosenRate = strings.Split((strings.Split(t.LastResponse.Text, `data-shipping-method="`)[1]), `"`)[0]
			log.Println("[SHOPIFY] Shipping rate chosen:", chosenRate)
			break
		}
		if iter > 10 {
			log.Println("[SHOPIFY] Passed 10 iterations. Shipping rate not found")
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		}
		t.LastResponse, err = t.Session.Get(t.SessionUrl+"/shipping_rates?step=shipping_method", headers, nil)
		if err != nil {
			log.Println("[SHOPIFY] error with request (shipping rate):", err)
			log.Println("[SHOPIFY] Rotating Proxy")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			t.Session.SetProxy(newProxy)
			errs.HandleErrorStatus(t, errs.ErrorWithRequest)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			continue
			// return errs.ErrorWithRequest
		}

		switch t.LastResponse.StatusCode() {
		case 202: // Waiting for shipping rates
			continue
		case 200: // Shipping rates found
			continue
		default: //Assume Proxy error
			log.Println("[SHOPIFY] unexpected status code (shipping rate):", t.LastResponse.StatusCode())
			log.Println("[SHOPIFY] Rotating Proxy")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			t.Session.SetProxy(newProxy)
			errs.HandleErrorStatus(t, errs.ErrorWithRequest)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			// return errs.ErrorWithRequest
		}
	}
	payload := url.Values{}
	payload.Set("_method", "patch")
	payload.Set("authenticity_token", t.AuthenticityToken)
	payload.Set("previous_step", "shipping_method")
	payload.Set("step", "payment_method")
	payload.Set("checkout[shipping_rate][id]", chosenRate)
	payload.Set("checkout[client_details][browser_width]", "1280")
	payload.Set("checkout[client_details][browser_height]", "720")
	payload.Set("checkout[client_details][javascript_enabled]", "1")
	payload.Set("checkout[client_details][color_depth]", "24")
	payload.Set("checkout[client_details][java_enabled]", "false")
	payload.Set("checkout[client_details][browser_tz]", "420")
	t.SetStatus("Submitting Shipping Rate", colors.Yellow)
	for {
		var resp requests.Response
		for {
			if !t.IsRunning() {
				return errs.ErrorTaskStop
			}
			headers = append(headers, map[string]string{"content-type": "application/x-www-form-urlencoded"})
			resp, err = t.Session.Post(t.SessionUrl, headers, payload.Encode())
			if err != nil {
				log.Println("[SHOPIFY] error with request (submit shipping rate):", err)
				log.Println("[SHOPIFY] Rotating Proxy")
				newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
				t.Session.SetProxy(newProxy)
				errs.HandleErrorStatus(t, errs.ErrorWithRequest)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			break
		}
		if strings.Contains(resp.Url(), "step") || strings.Contains(resp.Url(), "checkouts/") {
			t.LastResponse = resp
			break
		}
		log.Println("[SHOPIFY] Awaiting restock (submit shipping rate)", resp.Url())
		t.SetStatus("Awaiting Restock", colors.Yellow)
		time.Sleep(t.Settings.Shopify.MonitorDelay) //Monitor delay here bc waiting for restock
	}
	return nil
}
