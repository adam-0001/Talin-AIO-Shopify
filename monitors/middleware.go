package monitors

import (
	"fmt"
	"strings"

	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"golang.org/x/exp/slices"
)

// Extracts product information from a product .js payload based on desired sizes and a given strategy
func ExtractProductInfoJS(prod ProductDotJS, desiredSizes []string, strategy string) (shopifyTasks.CustomProduct, error) {
	var img, handle string
	if len(prod.Images) > 0 {
		// if !isJsEP {
		img = prod.Images[0]
		// } else {
		// 	img = "https" + prod.Images[0].Src
		// }
	} else if prod.FeaturedImage != "" {
		img = prod.FeaturedImage
	} else {
		img = emptyImg
	}
	handle = prod.Handle
	//Random size ✅
	var p Product
	p.Variants = prod.Variants
	p.Title = prod.Title
	if len(desiredSizes) == 0 || slices.Contains(desiredSizes, "random") || slices.Contains(desiredSizes, "ra") {
		switch strategy {
		case "ra":
			return extractRandomSizeRA(p, img, handle)
		default:
			return extractRandomSizeFA(p, img, handle)
		}
	} else if len(desiredSizes) == 1 { // WANTS ONE SIZE✅
		return extractSingleSize(p, img, handle, desiredSizes[0])

	} else {
		//WANTS ANY OF MULTIPLE SIZES✅
		switch strategy {
		case "ra":
			return extractFromSizeListRA(p, img, handle, desiredSizes)
		default:
			return extractFromSizeListFA(p, img, handle, desiredSizes)
		}
	}
}

// Extracts product information from products.json payload based on desired sizes and a given strategy
func ExtractProductInfo(prod Product, desiredSizes []string, strategy string) (shopifyTasks.CustomProduct, error) {
	var img, handle string
	if len(prod.Images) > 0 {
		// if !isJsEP {
		img = prod.Images[0].Src
		// } else {
		// 	img = "https" + prod.Images[0].Src
		// }
	} else {
		img = emptyImg
	}
	handle = prod.Handle
	//Random size ✅
	var p Product
	p.Variants = prod.Variants
	p.Title = prod.Title
	if len(desiredSizes) == 0 || slices.Contains(desiredSizes, "random") || slices.Contains(desiredSizes, "ra") {
		switch strategy {
		case "ra":
			return extractRandomSizeRA(prod, img, handle)
		default:
			return extractRandomSizeFA(prod, img, handle)
		}
	} else if len(desiredSizes) == 1 { // WANTS ONE SIZE✅
		return extractSingleSize(p, img, handle, desiredSizes[0])

	} else {
		//WANTS ANY OF MULTIPLE SIZES✅
		switch strategy {
		case "ra":
			return extractFromSizeListRA(prod, img, handle, desiredSizes)
		default:
			return extractFromSizeListFA(prod, img, handle, desiredSizes)
		}
	}
}

// Extract variant info given a product and a variant
// Return the necessary product info for discord webhooks
func extractVariantInfo(p Product, v Variant, img, handle string) shopifyTasks.CustomProduct {
	var sizeTitle string
	if v.Option1 != v.Title {
		sizeTitle += v.Title + "/" + v.Option1
	} else {
		sizeTitle += v.Title
	}
	if v.Option2 == v.Option1 {
		sizeTitle = strings.Split(sizeTitle, "/"+v.Option1)[0]
	} else if v.Option2 != "" {
		sizeTitle += "/" + v.Option2
	}
	price := fmt.Sprint(v.Price)
	if !strings.Contains(price, ".") {
		price = price[0:len(price)-2] + "." + price[len(price)-2:]
	}
	return shopifyTasks.CustomProduct{
		Variant:       fmt.Sprint(v.ID),
		Title:         p.Title,
		Image:         img,
		Size:          sizeTitle,
		ProductHandle: handle,
		Price:         price,
	}
}
