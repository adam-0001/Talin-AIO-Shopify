package monitors

import (
	"log"
	"math/rand"
	"time"

	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/requests"
)

type ParseMethod string

var (

	// ErrNoProductSpecified = fmt.Errorf("no product specified")
	ParseFAOrder = ParseMethod("fa")
	ParseRAOrder = ParseMethod("ra")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Monitor products.json for keywords
func KeywordMonitor(t *shopifyTasks.ShopifyTask) (shopifyTasks.CustomProduct, error) {
	posKws, negKws := ParseKws(t.Keywords)
	var method string = "fa"
	if ParseMethod(t.ParseMethod) == ParseRAOrder {
		method = "ra"
	}
	client, err := requests.NewSession(t.Settings.Shopify.Timeout, t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
	if err != nil {
		log.Println("[Shopify] Monitor err (initializing client):", err)
		return shopifyTasks.CustomProduct{}, err
	}
	prod, err := FetchProducts(t, client, posKws, negKws)
	if err != nil && err != errs.ErrorTaskStop {
		log.Println("[Shopify] Monitor err (fetching products):", err)
		return shopifyTasks.CustomProduct{}, err
	}
	p, err := ExtractProductInfo(prod, t.DesiredSizes, method)
	if err != nil {
		log.Println("[Shopify] Monitor err (extracting product info):", err)
		return p, err
	}
	return p, nil
}

// Monitor a given product link
func LinkMonitor(t *shopifyTasks.ShopifyTask) (shopifyTasks.CustomProduct, error) {
	var method string = "fa"
	if ParseMethod(t.ParseMethod) == ParseRAOrder {
		method = "ra"
	}
	client, err := requests.NewSession(t.Settings.Shopify.Timeout, t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
	if err != nil {
		log.Println("[Shopify] Monitor err (initializing client):", err)
		return shopifyTasks.CustomProduct{}, err
	}
	prod, err := FetchProductFromLink(t, client)
	if err != nil && err != errs.ErrorTaskStop {
		log.Println("[Shopify] Monitor err (fetching products):", err)
		return shopifyTasks.CustomProduct{}, err
	}
	p, err := ExtractProductInfoJS(prod, t.DesiredSizes, method)
	if err != nil {
		log.Println("[Shopify] Monitor err (extracting product info):", err)
		return p, err
	}
	return p, nil
}

// Return monitor type
func CheckMonitorType(t *shopifyTasks.ShopifyTask) string {
	if t.Variant != "" {
		return "var"
	} else if t.Keywords != "" {
		return "kws"
	} else if t.ProductLink != "" {
		return "link"
	}
	return ""
}
