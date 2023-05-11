package steps

import (
	"errors"
	"log"
	"math/rand"
	"strings"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
)

var ErrUnsupportedStore = errors.New("unsupported graphql store")

func CreateCheckoutSession(t *shopifyTasks.ShopifyTask) error {
	headers := []map[string]string{
		{"User-Agent": utils.GetUserAgent()},
	}
	t.SetStatus("Getting Checkout Session", colors.Yellow)
	resp, err := t.Session.Get(t.SiteUrl+"/cart/checkout", headers, nil)
	if err != nil {
		log.Println("[SHOPIFY] error with request (CreateCheckoutSession):", err)
		if strings.Contains(err.Error(), "EOF") {
			if strings.Contains(t.SiteUrl, "https://www.") {
				t.SiteUrl = strings.Replace(t.SiteUrl, "https://www.", "https://", 1)
			} else {
				t.SiteUrl = strings.Replace(t.SiteUrl, "https://", "https://www.", 1)
			}
		}
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		return errs.ErrorWithRequest
	}
	t.CheckoutUrl = resp.Url()
	if strings.Contains(t.CheckoutUrl, "/c/") {
		log.Println("[SHOPIFY] unsupported graphql store found, unable to proceed: ", t.CheckoutUrl)
		return ErrUnsupportedStore
	}
	log.Println("[SHOPIFY] checkout url: ", t.CheckoutUrl)
	parseAuthTokenAndSiteKey(resp.Text, t)
	t.LastResponse = resp
	return nil
}
