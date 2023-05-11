package steps

import (
	"encoding/base64"
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
)

func FastFindApiToken(t *shopifyTasks.ShopifyTask, res string) {
	if t.FastApiToken == "" {
		if strings.Contains(t.LastResponse.Text, `"paymentInstruments":{"accessToken"`) {
			tmp := strings.Split(t.LastResponse.Text, "dynamicCheckoutPaymentInstrumentsConfig")
			if len(tmp) > 1 {
				pt1 := tmp[1]
				tmp = strings.Split(pt1, `"accessToken"`)
				if len(tmp) > 1 {
					pt2 := tmp[1]
					tmp = strings.Split(pt2, `:"`)
					if len(tmp) > 1 {
						pt3 := tmp[1]
						fapiToken := strings.Split(pt3, `"`)[0]
						t.FastApiToken = base64.StdEncoding.EncodeToString([]byte(fapiToken))

					}
				}
			}
		}
	}
}

func FastSetDeliveryInfo(t *shopifyTasks.ShopifyTask, retry bool) error {
	t.SessionUrl = strings.Split(t.CheckoutUrl, "/stock_problems")[0]
	tmp := strings.Split(t.SessionUrl, "/checkouts/")
	if len(tmp) > 1 {
		t.CheckoutToken = tmp[1]
	} else {
		return errors.New("checkout token not found!" + t.SessionUrl)
	}
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
		log.Println("[SHOPIFY] Got hashes:", cleanedHashes)

	}
	if !retry {
		go fetchPaymentToken(t)
		log.Println("[SHOPIFY] Launched payment token goroutine")

	}
	if t.AuthenticityToken == "" {
		log.Println("[SHOPIFY] Unsupported store (graphQL)")
		return ErrUnsupportedStore
	}
	log.Println("[SHOPIFY] Submitting Address")
	payload := url.Values{}

	payload.Set("_method", "patch")
	payload.Set("authenticity_token", t.AuthenticityToken)
	payload.Set("previous_step", "contact_information")
	payload.Set("step", "shipping_method")
	payload.Set("checkout[email]", t.Profile.Email)
	payload.Set(t.CheckoutToken+"-count", fmt.Sprint(len(cleanedHashes)))
	payload.Set(t.CheckoutToken+"-count", "fs_count")
	payload.Set("checkout[email]", t.Profile.Email)
	payload.Set("checkout[shipping_address][first_name]", t.Profile.ShippingInfo.FirstName)
	payload.Set("checkout[shipping_address][last_name]", t.Profile.ShippingInfo.LastName)
	payload.Set("checkout[shipping_address][address1]", t.Profile.ShippingInfo.ShippingAddress1)
	payload.Set("checkout[shipping_address][address2]", t.Profile.ShippingInfo.ShippingAddress2)
	payload.Set("checkout[shipping_address][city]", t.Profile.ShippingInfo.City)
	payload.Set("checkout[shipping_address][province]", t.Profile.ShippingInfo.State)
	payload.Set("checkout[shipping_address][country]", t.Profile.ShippingInfo.Country)
	payload.Set("checkout[shipping_address][zip]", t.Profile.ShippingInfo.ZipCode)
	payload.Set("checkout[shipping_address][phone]", t.Profile.Phone)
	payload.Set("checkout[remember_me]", "0")
	payload.Set("checkout[attributes][I-agree-to-the-Terms-and-Conditions]", "Yes")
	payload.Set("checkout[buyer_accepts_marketing]", "0")
	payload.Set("checkout[client_details][browser_width]", "1280")
	payload.Set("checkout[client_details][browser_height]", "720")
	payload.Set("checkout[client_details][javascript_enabled]", "1")
	payload.Set("checkout[client_details][color_depth]", "24")
	payload.Set("checkout[client_details][java_enabled]", "false")
	payload.Set("checkout[client_details][browser_tz]", "420")
	for _, item := range cleanedHashes {
		payload.Set(item, "")
	}
	headers := []map[string]string{
		{"user-agent": utils.GetUserAgent()},
		{"content-type": "application/x-www-form-urlencoded"},
		{"authorization": "Basic " + t.FastApiToken},
	}
	for {
		if !t.IsRunning() {
			return errs.ErrorTaskStop
		}
		res, err := t.Session.Patch(t.SiteUrl+"/wallets/checkouts/"+t.CheckoutToken+".json", headers, payload.Encode())
		if err != nil {
			log.Println("[SHOPIFY] error with request (delivery):", err)
			log.Println("[SHOPIFY] Rotating Proxy")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			t.Session.SetProxy(newProxy)
			return errs.ErrorWithRequest
		}
		if res.StatusCode() != 200 && res.StatusCode() != 202 {
			if strings.Contains(res.Url(), "stock_problems") {
				log.Println("[SHOPIFY] Product out of stock - Awaiting Restock")
				t.SetStatus("Awaiting Restock", colors.Yellow)
				newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
				t.Session.SetProxy(newProxy)
				time.Sleep(t.Settings.Shopify.MonitorDelay)
				continue
			}
			log.Println("[SHOPIFY] unexpected status code (delivery):", res.StatusCode())
			t.SetStatus("Retrying Shipping Submission ("+res.Status()+")", colors.Yellow)
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			t.Session.SetProxy(newProxy)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		} else {
			return nil
		}
	}
}

func FastSetShippingRate(t *shopifyTasks.ShopifyTask) error {
	headers := []map[string]string{
		{"user-agent": utils.GetUserAgent()},
		{"authorization": "Basic " + t.FastApiToken},
	}
	t.SetStatus("Fetching Shipping Rates", colors.Yellow)
	req, err := t.Session.Get(t.SiteUrl+"/api/2021-10/checkouts/"+t.CheckoutToken+"/shipping_rates.json", headers, nil)
	if err != nil {
		log.Println("[SHOPIFY] error with request (delivery):", err)
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		time.Sleep(t.Settings.Shopify.ErrorDelay)
		return errs.ErrorWithRequest
	}
	if req.StatusCode() != 200 {
		log.Println("[SHOPIFY] error fetching shipping rates (fast)")
		t.SetStatus("Error Fetching Shipping Rates - Retrying", colors.Red)
		time.Sleep(t.Settings.Shopify.ErrorDelay)
		return errors.New("error fetching shipping rates - " + req.Status())
	}
	type shippingRes struct {
		ShippingRates []struct {
			Id string `json:"id"`
		} `json:"shipping_rates"`
	}
	var (
		selectedRate string
		noNeed       bool
		sRes         shippingRes
	)
	err = req.Json(&sRes)
	if err != nil {
		noNeed = true
	}
	if len(sRes.ShippingRates) < 1 {
		noNeed = true
	} else {
		selectedRate = sRes.ShippingRates[0].Id
	}
	headers = append(headers, map[string]string{"content-type": "application/json"})
	if !noNeed {
		log.Println("[SHOPIFY] Submitting Shipping")
		form := map[string]interface{}{
			"checkout": map[string]interface{}{
				"token": t.CheckoutToken,
				"shipping_line": map[string]interface{}{
					"handle": selectedRate,
				},
			},
		}
		for {
			t.SetStatus("Submitting Shipping", colors.Yellow)
			if !t.IsRunning() {
				return errs.ErrorTaskStop
			}
			res, err := t.Session.Put(t.SiteUrl+"/wallets/checkouts/"+t.CheckoutToken+".json", headers, form)
			if err != nil {
				log.Println("[SHOPIFY] error with request (shipping):", err)
				log.Println("[SHOPIFY] Rotating Proxy")
				newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
				t.Session.SetProxy(newProxy)
				errs.HandleErrorStatus(t, errs.ErrorWithRequest)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			if res.StatusCode() != 200 && res.StatusCode() != 202 {
				log.Println("[SHOPIFY] error submitting shipping", res.StatusCode())
				t.SetStatus("Error Submiting Shipping - Retrying", colors.Red)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			} else {
				break
			}
		}
	}
	return nil
}
