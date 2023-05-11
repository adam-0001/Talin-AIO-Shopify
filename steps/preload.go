package steps

import (
	"encoding/json"
	"errors"
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

func PreloadFetchVariant(t *shopifyTasks.ShopifyTask) (string, error) {
	var (
		limit int = 1
		resp  requests.Response
		err   error
	)
	monitorEp := func() string {
		newLim := float64(limit) + rand.Float64()
		return fmt.Sprintf(t.SiteUrl+"//products.json?limit=%v", newLim)
	}
	client, err := requests.Client(t.Settings.Shopify.Timeout, t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
	if err != nil {
		return "", err
	}
	for {
		t.SetStatus("Fetching Preload Item", colors.Yellow)
		if !t.IsRunning() {
			return "", errs.ErrorTaskStop
		}
		resp, err = client.Get(monitorEp(), nil, nil)
		if err != nil {
			return "", err
		}
		if resp.StatusCode() != 200 {
			switch resp.StatusCode() {
			case 401:
				log.Println("[Shopify] Monitor err (will continue) (password):", resp.StatusCode())
				t.SetStatus("Password Protected - Retrying", colors.Red)

			case 430:
				log.Println("[Shopify] Monitor err (will continue) (rate limit):", resp.StatusCode())
				client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
				t.SetStatus("Monitor Rate Limit - Retrying", colors.Red)
			default:
				log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				t.SetStatus("Monitor Error ("+fmt.Sprint(resp.StatusCode())+") - Retrying", colors.Red)

				client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
			}
			log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
			t.SetStatus("Error Monitoring - "+fmt.Sprint(resp.StatusCode()), colors.Red)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			continue
		}
		break
	}
	if strings.Contains(resp.Text, `"variants"`) {
		p1 := strings.Split(resp.Text, `"variants"`)[1]
		if p2 := strings.Split(p1, `{"id":`); len(p2) > 1 {
			return strings.Split(p2[1], `,`)[0], nil
		}
		return "", errors.New("error parsing variant")
	}
	return "", errors.New("no variants found")
}

func PreloadClearCart(t *shopifyTasks.ShopifyTask, variant string) error {
	url := fmt.Sprintf("%s/cart/clear.js", t.SiteUrl)
	resp, err := t.Session.Post(url, nil, nil)
	if err != nil {
		log.Println("[SHOPIFY] error with request (clear cart):", err)
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		return errs.ErrorWithRequest
	}
	if resp.StatusCode() == 200 {
		return nil
	} else {
		return errors.New(resp.Status() + " error clearing cart")
	}
}

func PreloadAtc(t *shopifyTasks.ShopifyTask, variant string) error {
	var atcResp atcResponse
	form := fmt.Sprintf("id=%v", variant)
	url := fmt.Sprintf("%s/cart/add.js", t.SiteUrl)
	resp, err := t.Session.Post(url, nil, form)
	if err != nil {
		log.Println("[SHOPIFY] error with request (atc):", err)
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		return errs.ErrorWithRequest
	}
	if resp.StatusCode() == 200 {
		err := json.Unmarshal([]byte(resp.Text), &atcResp)
		if err != nil {
			err := errors.New(fmt.Sprint("err shopify ATC:", err))
			return err
		}
		if atcResp.ID != 0 {
			log.Println("[SHOPIFY] Successfully added to cart")
			return nil
		} else {
			err := fmt.Errorf("zero value for ATC response : %v", atcResp.ID)
			return err
		}
	} else if resp.StatusCode() == 404 {
		return fmt.Errorf("err ATC (product not found): %v", resp.StatusCode())
	} else {
		return fmt.Errorf("err ATC: %v", resp.StatusCode())
	}
}

func PreloadSetShippingRate(t *shopifyTasks.ShopifyTask) error {
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
	headers = append(headers, map[string]string{"content-type": "application/x-www-form-urlencoded"})
	for {
		var resp requests.Response
		for {
			t.SetStatus("Setting Shipping Rate", colors.Yellow)
			if !t.IsRunning() {
				return errs.ErrorTaskStop
			}
			resp, err = t.Session.Post(t.SessionUrl, headers, payload.Encode())
			if err != nil || resp.StatusCode() != 200 {
				log.Println("[SHOPIFY] error with request (submit shipping rate):", err)
				log.Println("[SHOPIFY] Rotating Proxy")
				newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
				t.Session.SetProxy(newProxy)
				if err != nil {
					errs.HandleErrorStatus(t, errs.ErrorWithRequest)
				} else {
					t.SetStatus(fmt.Sprintf("Error Submitting Shipping Rate (%v)", resp.StatusCode()), colors.Red)
				}
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			break
		}
		return nil
	}
}
