package monitors

import (
	"errors"
	"log"
	"math/rand"
	"strconv"
	"strings"

	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
)

var emptyImg = "https://go-webhook-middleman.herokuapp.com/static/webhook-icon.png"

// Parse input keywords
func ParseKws(in string) (pos, neg []string) {
	split := strings.Split(strings.ToLower(strings.TrimSpace(in)), ",")
	for _, s := range split {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "-") {
			neg = append(neg, s[1:])
		} else {
			pos = append(pos, s)
		}
	}
	return
}

// Check if title contains any of the keywords
func checkForKWMatch(title string, pos, neg []string) bool {
	title = strings.ToLower(title)
	for _, kw := range neg {
		if strings.Contains(title, kw) {
			return false
		}
	}
	for _, kw := range pos {
		if strings.Contains(title, kw) {
			return true
		}
	}
	return false
}

// Check if item size is the same as the desired size
func checkForSizeMatch(desired, title, opt1, opt2 string) bool {
	title, opt1, opt2, desired = strings.ToLower(title), strings.ToLower(opt1), strings.ToLower(opt2), strings.ToLower(desired)
	actual := title + opt1 + opt2
	if !strings.Contains(actual, desired) {
		return false
	}
	if desired == title || desired == opt1 {
		return true
	}

	_, err := strconv.ParseFloat(desired, 32)
	isNum := err == nil
	if isNum {
		_, err := strconv.ParseInt(desired, 10, 0)
		isInteger := err == nil
		if isInteger {
			if strings.Contains(actual, ".") {
				return false
			}
		}
		return true
	}
	switch desired {
	case "xxsmall":
		return strings.Contains(actual, "xxs") || strings.Contains(actual, desired) || strings.Contains(actual, "xtra extra small") || strings.Contains(actual, "xtra") && strings.Contains(actual, "small") || strings.Contains(actual, "xx") && strings.Contains(actual, "s") && !strings.Contains(actual, "large") && !strings.Contains(actual, "xxl")
	case "xsmall":
		return strings.Contains(actual, "xsmall") || strings.Contains(actual, "xs") || strings.Contains(actual, "xtra small") || strings.Contains(actual, "xtra") && strings.Contains(actual, "small") && !strings.Contains(actual, "xx")
	case "small":
		return (strings.Contains(actual, "s") || strings.Contains(actual, desired)) && !strings.Contains(actual, "xtra") && !strings.Contains(actual, "x")
	case "medium":
		return strings.Contains(actual, "m") || strings.Contains(actual, desired)
	case "large":
		return (strings.Contains(actual, "l") || strings.Contains(actual, desired)) && (!strings.Contains(actual, "xtra") && !strings.Contains(actual, "x"))
	case "xlarge":
		return strings.Contains(actual, "xlarge") || strings.Contains(actual, "xl") || strings.Contains(actual, "xtra large") || strings.Contains(actual, "xtra") && strings.Contains(actual, "large") && !strings.Contains(actual, "xx")
	case "xxlarge":
		return strings.Contains(actual, "xxl") || strings.Contains(actual, desired) || strings.Contains(actual, "xtra extra large") || strings.Contains(actual, "xtra") && strings.Contains(actual, "large") || strings.Contains(actual, "xx") && strings.Contains(actual, "l") && !strings.Contains(actual, "small") && !strings.Contains(actual, "s")
	}
	return true
}

// Extracts random size variant given a product's list of variants
// If a matching variant is found, it is added to a list of available variants and a random variant is returned
// Returns ErrOutOfStock if no available variants are found
func extractRandomSizeRA(prod Product, img, handle string) (shopifyTasks.CustomProduct, error) {
	variants := prod.Variants
	if len(variants) != 0 {
		copyOfVariants := variants
		for len(copyOfVariants) != 0 {
			ind := rand.Intn(len(copyOfVariants))
			v := copyOfVariants[ind]
			if v.Available {
				return extractVariantInfo(prod, v, img, handle), nil
			} else {
				copyOfVariants[ind] = copyOfVariants[len(copyOfVariants)-1]
				copyOfVariants = copyOfVariants[:len(copyOfVariants)-1]
			}
		}
		log.Println("[Shopify] No available variants found - selecting random size")
		return extractVariantInfo(prod, variants[rand.Intn(len(variants))], img, handle), errs.ErrOutOfStock
	} else {
		return shopifyTasks.CustomProduct{}, errors.New("cannot extract size from empty list")
	}
}

// Extracts a random size variant given a product's list of variants
// If a matching variant is found, it is immediately returned
func extractRandomSizeFA(prod Product, img, handle string) (shopifyTasks.CustomProduct, error) {
	variants := prod.Variants
	if len(variants) != 0 {
		copyOfVariants := variants
		for _, variant := range copyOfVariants {
			if variant.Available {
				return extractVariantInfo(prod, variant, img, handle), nil
			}
		}
		log.Println("[Shopify] No available variants found - selecting random size")
		return extractVariantInfo(prod, variants[0], img, handle), errs.ErrOutOfStock
	} else {
		return shopifyTasks.CustomProduct{}, errors.New("cannot extract size from empty list")
	}
}

// Extracts a specific size variant given a product's list of variants based on a desired size
// If a matching variant is found, it is immediately returned
func extractSingleSize(prod Product, img, handle, desired string) (shopifyTasks.CustomProduct, error) {
	variants := prod.Variants
	if len(variants) != 0 {
		for _, variant := range variants {
			if checkForSizeMatch(desired, variant.Title, variant.Option1, variant.Option2) {
				log.Println("[Shopify] Found matching variant:", variant.Title, "(want: "+desired+")")
				if variant.Available {
					return extractVariantInfo(prod, variant, img, handle), nil
				} else {
					return extractVariantInfo(prod, variant, img, handle), errs.ErrOutOfStock
				}
			}
		}
		return shopifyTasks.CustomProduct{}, errs.ErrSizeNotFound
	} else {
		return shopifyTasks.CustomProduct{}, errors.New("cannot extract size from empty list")
	}
}

// Extract a random size variant given a product's list of variants based on a list of desired sizes
// Sizes are chosen randomly from the list of desired sizes
// Iterates through desired sizes and selects a random size from the list and searches for a matching variant
// If a matching variant is found, it is immediately returned
// If no in stock variants are found, the first out of stock variant is returned along with ErrOutOfStock error
// If no matching variants are found, it returns an empty result with ErrSizeNotFound error
func extractFromSizeListRA(prod Product, img, handle string, desired []string) (shopifyTasks.CustomProduct, error) {
	variants := prod.Variants
	if len(variants) != 0 {
		oos := []Variant{}
		for len(desired) != 0 {
			choiceIndex := rand.Intn(len(desired))
			chosen := desired[choiceIndex]
			for _, variant := range variants {
				if checkForSizeMatch(chosen, variant.Title, variant.Option1, variant.Option2) {
					log.Println("[Shopify] Found matching variant:", variant.Title, "(want: "+chosen+")")
					if variant.Available {
						return extractVariantInfo(prod, variant, img, handle), nil
					} else {
						oos = append(oos, variant)
					}
				} else {
					desired[choiceIndex] = desired[len(desired)-1]
					desired = desired[:len(desired)-1]
				}
			}
		}
		if len(oos) != 0 {
			log.Println("[Shopify] No in stock options found - selecting oos option")
			return extractVariantInfo(prod, oos[0], img, handle), errs.ErrOutOfStock
		}
		log.Println("[Shopify] No matching variants found - returning empty")
		return shopifyTasks.CustomProduct{}, errs.ErrSizeNotFound
	} else {
		return shopifyTasks.CustomProduct{}, errors.New("cannot extract size from empty list")
	}

}

// Extracts the first available size variant given a product's list of variants based on a list of desired sizes
// Iterates through desired sizes and searches for a matching variant
// If a matching variant is found, it is immediately returned
// If no in stock variants are found, the first out of stock variant is returned along with ErrOutOfStock error
// If no matching variants are found, it returns an empty result with ErrSizeNotFound error
func extractFromSizeListFA(prod Product, img, handle string, desired []string) (shopifyTasks.CustomProduct, error) {
	variants := prod.Variants
	if len(variants) != 0 {
		oos := []Variant{}
		for _, choice := range desired {
			for _, variant := range variants {
				if checkForSizeMatch(choice, variant.Title, variant.Option1, variant.Option2) {
					log.Println("[Shopify] Found matching variant:", variant.Title, "(want: "+choice+")")
					if variant.Available {
						return extractVariantInfo(prod, variant, img, handle), nil
					} else {
						oos = append(oos, variant)
					}
				}
			}
		}
		if len(oos) != 0 {
			log.Println("[Shopify] No in stock options found - selecting oos option")
			return extractVariantInfo(prod, oos[0], img, handle), errs.ErrOutOfStock
		}
		log.Println("[Shopify] No matching variants found - returning empty")
		return shopifyTasks.CustomProduct{}, errs.ErrSizeNotFound
	} else {
		return shopifyTasks.CustomProduct{}, errors.New("cannot extract size from empty list")
	}

}
