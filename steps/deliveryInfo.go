package steps

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"strings"

	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
)

func SetDeliveryInfo(t *shopifyTasks.ShopifyTask, retry bool) error {
	t.SessionUrl = strings.Split(strings.Split(t.CheckoutUrl, "?")[0], "stock_problems")[0]
	if checkoutUrlParts := strings.Split(t.CheckoutUrl, "/checkouts/"); len(checkoutUrlParts) > 1 {
		t.CheckoutToken = checkoutUrlParts[1]
	} else {
		return errors.New("error getting checkout token")
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
	log.Println("[SHOPIFY] Submitting Address")
	headers := []map[string]string{
		{"user-agent": utils.GetUserAgent()},
		{"content-type": "application/x-www-form-urlencoded"},
	}
	payload := url.Values{}

	payload.Set("_method", "patch")
	payload.Set("authenticity_token", t.AuthenticityToken)
	payload.Set("previous_step", "contact_information")
	payload.Set("step", "shipping_method")
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
	payload.Set(t.CheckoutToken+"-count", fmt.Sprint(len(cleanedHashes)))
	payload.Set(t.CheckoutToken+"-count", "fs_count")

	for _, item := range cleanedHashes {
		payload.Set(item, "")
	}
	resp, err := t.Session.Post(t.SessionUrl, headers, payload.Encode())
	if err != nil {
		log.Println("[SHOPIFY] error with request (delivery):", err)
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		return errs.ErrorWithRequest
	}
	if resp.StatusCode() >= 400 {
		log.Println("[SHOPIFY] unexpected status code (delivery):", resp.StatusCode())
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		return errs.ErrorWithRequest
	}
	t.LastResponse = resp
	return nil
}
