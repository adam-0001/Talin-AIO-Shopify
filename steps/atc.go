package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"

	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
)

type atcResponse struct {
	ID         int64 `json:"id"`
	Properties struct {
		Upsell string `json:"upsell"`
	} `json:"properties"`
	Quantity                     int           `json:"quantity"`
	VariantID                    int64         `json:"variant_id"`
	Key                          string        `json:"key"`
	Title                        string        `json:"title"`
	Price                        int           `json:"price"`
	OriginalPrice                int           `json:"original_price"`
	DiscountedPrice              int           `json:"discounted_price"`
	LinePrice                    int           `json:"line_price"`
	OriginalLinePrice            int           `json:"original_line_price"`
	TotalDiscount                int           `json:"total_discount"`
	Discounts                    []interface{} `json:"discounts"`
	Sku                          string        `json:"sku"`
	Grams                        int           `json:"grams"`
	Vendor                       string        `json:"vendor"`
	Taxable                      bool          `json:"taxable"`
	ProductID                    int64         `json:"product_id"`
	ProductHasOnlyDefaultVariant bool          `json:"product_has_only_default_variant"`
	GiftCard                     bool          `json:"gift_card"`
	FinalPrice                   int           `json:"final_price"`
	FinalLinePrice               int           `json:"final_line_price"`
	URL                          string        `json:"url"`
	FeaturedImage                struct {
		AspectRatio float64 `json:"aspect_ratio"`
		Alt         string  `json:"alt"`
		Height      int     `json:"height"`
		URL         string  `json:"url"`
		Width       int     `json:"width"`
	} `json:"featured_image"`
	Image              string   `json:"image"`
	Handle             string   `json:"handle"`
	RequiresShipping   bool     `json:"requires_shipping"`
	ProductType        string   `json:"product_type"`
	ProductTitle       string   `json:"product_title"`
	ProductDescription string   `json:"product_description"`
	VariantTitle       string   `json:"variant_title"`
	VariantOptions     []string `json:"variant_options"`
	OptionsWithValues  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"options_with_values"`
	LineLevelDiscountAllocations []interface{} `json:"line_level_discount_allocations"`
	LineLevelTotalDiscount       int           `json:"line_level_total_discount"`
}

func Atc(t *shopifyTasks.ShopifyTask, variant string) error {
	var atcResp atcResponse
	form := fmt.Sprintf("id=%v&quantity=%v", variant, t.Quantity)
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
			if t.Variant != "" {
				pr := fmt.Sprint(atcResp.Price)
				if len(pr) > 2 {
					t.Product.Price = pr[:len(pr)-2] + "." + pr[len(pr)-2:]
				} else {
					t.Product.Price = pr
				}
				t.Product.Image = atcResp.Image
				t.Product.ProductHandle = atcResp.Handle
				t.Product.Size = atcResp.VariantTitle
				t.Product.Title = atcResp.ProductTitle
				t.Product.Variant = t.Variant
			}
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
