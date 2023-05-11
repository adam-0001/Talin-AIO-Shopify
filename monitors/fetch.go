package monitors

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/requests"
)

// Main monitoring function
// Gets products from products.json and checks for keyword matches
func FetchProducts(t *shopifyTasks.ShopifyTask, client *requests.Session, posKws, negKws []string) (Product, error) {
	var limit int
	var productsJson = &ProductsDotJson{}
	if t.ReleaseMode {
		limit = 35
	} else {
		limit = 251
	}
	indexingApi := limit > 250
	monitorEp := func() string {
		//Bypass cache by adding random partial number to limit
		newLim := float64(limit) + rand.Float64()
		return fmt.Sprintf(t.SiteUrl+"//products.json?limit=%v&page=", newLim)
	}
	if indexingApi {
		var currentPage = 1
		for {
			t.SetStatus("Monitoring Product", colors.Yellow)
			if !t.IsRunning() {
				return Product{}, errs.ErrorTaskStop
			}
			log.Println("[Shopify] Indexing API - ", currentPage)
			resp, err := client.Get(monitorEp()+fmt.Sprint(currentPage), nil, nil)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue):", err)

				errs.HandleErrorStatus(t, errs.ErrorWithRequest)
				time.Sleep(t.Settings.Shopify.ErrorDelay)

				continue
			}
			if resp.StatusCode() != 200 {
				switch resp.StatusCode() {
				case 401:
					log.Println("[Shopify] Monitor err (will continue) (password):", resp.StatusCode())

				case 430:
					log.Println("[Shopify] Monitor err (will continue) (rate limit):", resp.StatusCode())
					client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
				default:
					log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
					client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
				}
				log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			err = resp.Json(productsJson)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue) (error decoding JSON):", err)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			products := productsJson.Products
			if len(products) == 0 {
				if currentPage == 1 {
					log.Println("[Shopify] Monitor err (no products found):", err)
					return Product{}, errors.New("no products found")
				} else {
					log.Println("[Shopify] Monitor err (no products found):", err)
					time.Sleep(t.Settings.Shopify.MonitorDelay)
					currentPage = 1
					continue
				}
			}
			for _, product := range products {
				if checkForKWMatch(product.Title, posKws, negKws) {
					log.Println("[Shopify] Monitor product found: ", product.Title)
					return product, nil
				}
			}
			currentPage++
			time.Sleep(t.Settings.Shopify.MonitorDelay)
		}
	} else {
		for {
			t.SetStatus("Monitoring Product", colors.Yellow)
			if !t.IsRunning() {
				return Product{}, errs.ErrorTaskStop
			}
			resp, err := client.Get(monitorEp(), nil, nil)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue):", err)
				client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
				errs.HandleErrorStatus(t, errs.ErrorWithRequest)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			if resp.StatusCode() != 200 {
				switch resp.StatusCode() {
				case 401:
					log.Println("[Shopify] Monitor err (will continue) (password):", resp.StatusCode())

				case 430:
					log.Println("[Shopify] Monitor err (will continue) (rate limit):", resp.StatusCode())
					client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
				default:
					log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
					client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
				}
				log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			err = resp.Json(productsJson)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue) (error decoding JSON):", err)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				continue
			}
			products := productsJson.Products
			for _, product := range products {
				if checkForKWMatch(product.Title, posKws, negKws) {
					log.Println("[Shopify] Monitor product found: ", product.Title)

					return product, nil
				}
			}
			time.Sleep(t.Settings.Shopify.MonitorDelay)
			log.Println("[Shopify] Monitor err (product not found):")
		}
	}

}

// Get product JSON given a product link (using .js endpoint)
func FetchProductFromLink(t *shopifyTasks.ShopifyTask, client *requests.Session) (ProductDotJS, error) {
	var limit = 35
	var productsJson ProductDotJS
	link := strings.ReplaceAll(t.ProductLink, "www.", "") + ".js"
	if !strings.Contains(link, "https://") {
		link = "https://" + link
	}
	monitorEp := func() string {
		newLim := float64(limit) + rand.Float64()
		return fmt.Sprintf(link+"?limit=%v", newLim)
	}
	for {
		t.SetStatus("Monitoring Product", colors.Yellow)

		if !t.IsRunning() {
			return productsJson, errs.ErrorTaskStop
		}
		resp, err := client.Get(monitorEp(), nil, nil)
		if err != nil {
			log.Println("[Shopify] Monitor err (will continue):", err)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			errs.HandleErrorStatus(t, errs.ErrorWithRequest)
			client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
			continue
		}
		if resp.StatusCode() != 200 {
			switch resp.StatusCode() {
			case 401:
				log.Println("[Shopify] Monitor err (will continue) (password):", resp.StatusCode())

			case 430:
				log.Println("[Shopify] Monitor err (will continue) (rate limit):", resp.StatusCode())
				client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
			default:
				log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				client.SetProxy(t.MonitorProxies[rand.Intn(len(t.MonitorProxies))])
			}
			log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
			t.SetStatus("Error Monitoring - "+fmt.Sprint(resp.StatusCode()), colors.Red)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			continue
		}
		err = resp.Json(&productsJson)
		if err != nil {
			log.Println("[Shopify] Monitor err (will continue) (error decoding JSON):", err)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			continue
		}
		return productsJson, nil

	}
}
