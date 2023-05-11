package monitors

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"testing"
	"time"

	errs "github.com/adam-0001/go-talin/errors"
	"github.com/adam-0001/requests"
)

func TestKwMon(t *testing.T) {
	// posKws, negKws := ParseKws("nike dunk low      , nike dunk low red, dunk low red, air jordan 4, -preschool, -gs, -ps, -gradeschool")
	posKws, negKws := ParseKws("union dunk, dunk low, dunk, -ps, -gs")
	// desiredSizes := []string{"12", "13.5", "11", "10.5", "10", "9.5", "9", "8.5", "8", "7.5", "7", "6.5", "6", "5.5", "5", "4.5", "3.5"}
	desiredSizes := []string{}
	var method string = "fa"
	client, err := requests.NewSession(10*time.Second, "")
	if err != nil {
		log.Println("[Shopify] Monitor err (initializing client):", err)
	}
	prod, err := fetchProductsTest("https://undefeated.com", client, posKws, negKws, true)
	if err != nil && err != errs.ErrorTaskStop {
		log.Println("[Shopify] Monitor err (fetching products):", err)
	}
	item, err := ExtractProductInfo(prod, desiredSizes, method)
	log.Println(item.ProductHandle)
	log.Println(item.Variant)
	if err != nil {
		log.Println("[Shopify] Monitor err (extracting product info):", err)
	}
}
func TestLnkMon(t *testing.T) {
	// posKws, negKws := ParseKws("nike dunk low      , nike dunk low red, dunk low red, air jordan 4, -preschool, -gs, -ps, -gradeschool")
	// desiredSizes := []string{"12", "13.5", "11", "10.5", "10", "9.5", "9", "8.5", "8", "7.5", "7", "6.5", "6", "5.5", "5", "4.5", "3.5"}
	desiredSizes := []string{}
	lnk := `https://www.kith.com/collections/stone-island-marina/products/simo7615661x7-v0044`
	var method string = "fa"
	client, err := requests.NewSession(10*time.Second, "")
	if err != nil {
		log.Println("[Shopify] Monitor err (initializing client):", err)
	}
	prod, err := fetchProductFromLinkTest(lnk, client)
	if err != nil {
		log.Println("[Shopify] Monitor err (fetching products):", err)
	}
	item, err := ExtractProductInfoJS(prod, desiredSizes, method)
	log.Println(item.ProductHandle)
	if err != nil {
		log.Println("[Shopify] Monitor err (extracting product info):", err)
	}
	log.Println(item.Price)
	log.Println(item.Size)
}

// func TestFetchProducts(t *testing.T) {
// 	c, _ := requests.NewSession(10*time.Second, "")
// 	url := "https://kith.com"
// 	for i := 0; i < 45; i++ {
// 		res, err := FpReqTest(url, c)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		t.Log(res)
// 	}
// }

// func FpReqTest(siteUrl string, client *requests.Session) (string, error) {
// 	var limit int
// 	monitorEp := func() string {
// 		newLim := float64(limit) + rand.Float64()
// 		return fmt.Sprintf(siteUrl+"//products.json?limit=%v&page=", newLim)
// 	}
// 	res, err := client.Get(monitorEp(), nil, nil)
// 	if err != nil {
// 		return "", err
// 	}
// 	return res.Headers()["X-Cache"], nil
// }

func fetchProductFromLinkTest(prod string, client *requests.Session) (ProductDotJS, error) {
	var limit = 35
	var productsJson ProductDotJS
	link := strings.ReplaceAll(prod, "www.", "") + ".js"
	if !strings.Contains(link, "https://") {
		link = "https://" + link
	}
	monitorEp := func() string {
		newLim := float64(limit) + rand.Float64()
		return fmt.Sprintf(link+"?limit=%v", newLim)
	}
	for {
		resp, err := client.Get(monitorEp(), nil, nil)
		if err != nil {
			log.Println("[Shopify] Monitor err (will continue):", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode() != 200 {
			switch resp.StatusCode() {
			case 401:
				log.Println("[Shopify] Monitor err (will continue) (password):", resp.StatusCode())

			case 430:
				log.Println("[Shopify] Monitor err (will continue) (rate limit):", resp.StatusCode())
			default:
				log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())

			}
			log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
			time.Sleep(2 * time.Second)
			continue
		}
		err = resp.Json(&productsJson)
		if err != nil {
			log.Println("[Shopify] Monitor err (will continue) (error decoding JSON):", err)
			time.Sleep(2 * time.Second)
			continue
		}
		return productsJson, nil

	}
}

func fetchProductsTest(siteUrl string, client *requests.Session, posKws, negKws []string, releaseMode bool) (Product, error) {
	var limit int
	var productsJson = &ProductsDotJson{}
	if releaseMode {
		limit = 35
	} else {
		limit = 10000
	}
	indexingApi := limit > 250
	monitorEp := func() string {
		newLim := float64(limit) + rand.Float64()
		return fmt.Sprintf(siteUrl+"//products.json?limit=%v&page=", newLim)
	}
	if indexingApi {
		var currentPage = 1
		for {
			log.Println("[Shopify] Indexing API - ", currentPage)
			resp, err := client.Get(monitorEp()+fmt.Sprint(currentPage), nil, nil)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue):", err)
				time.Sleep(3 * time.Second)
				continue
			}
			if resp.StatusCode() != 200 {
				switch resp.StatusCode() {
				case 401:
					log.Println("[Shopify] Monitor err (will continue) (password):", resp.StatusCode())

				case 430:
					log.Println("[Shopify] Monitor err (will continue) (rate limit):", resp.StatusCode())
				default:
					log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				}
				log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				time.Sleep(3 * time.Second)

				continue
			}
			err = resp.Json(productsJson)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue) (error decoding JSON):", err)
				time.Sleep(3 * time.Second)
				continue
			}
			products := productsJson.Products
			if len(products) == 0 {
				if currentPage == 1 {
					log.Println("[Shopify] Monitor err (no products found):", err)
					return Product{}, errors.New("no products found")
				} else {
					log.Println("[Shopify] Monitor err (no products found):", err)
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
			time.Sleep(3 * time.Second)

		}
	} else {
		for {
			resp, err := client.Get(monitorEp(), nil, nil)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue):", err)
				time.Sleep(3 * time.Second)
				continue
			}
			if resp.StatusCode() != 200 {
				switch resp.StatusCode() {
				case 401:
					log.Println("[Shopify] Monitor err (will continue) (password):", resp.StatusCode())

				case 430:
					log.Println("[Shopify] Monitor err (will continue) (rate limit):", resp.StatusCode())
				default:
					log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				}
				log.Println("[Shopify] Monitor err (will continue):", resp.StatusCode())
				time.Sleep(3 * time.Second)
				continue
			}
			err = resp.Json(productsJson)
			if err != nil {
				log.Println("[Shopify] Monitor err (will continue) (error decoding JSON):", err)
				time.Sleep(3 * time.Second)
				continue
			}
			products := productsJson.Products
			for _, product := range products {
				if checkForKWMatch(product.Title, posKws, negKws) {
					log.Println("[Shopify] Monitor product found: ", product.Title)

					return product, nil
				}
			}
			log.Println("[Shopify] Monitor err (product not found):")
		}
	}

}
