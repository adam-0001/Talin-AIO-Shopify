package steps

import (
	"strings"

	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
)

func parseAuthTokenAndSiteKey(response string, t *shopifyTasks.ShopifyTask) {
	if strings.Contains(response, "authenticity_token") {
		t.AuthenticityToken = strings.Split(strings.Split(strings.Split(response, "authenticity_token")[1], `value="`)[1], `"`)[0]
		// log.Println("[SHOPIFY] New Authenticity Token Found: " + t.AuthenticityToken)
	}
	if strings.Contains(response, `sitekey: "`) {
		t.Sitekey = strings.Split((strings.Split(response, `sitekey: "`)[1]), `"`)[0]
		// log.Println("[SHOPIFY] New Sitekey Found: " + t.Sitekey)
	}
}
