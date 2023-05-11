package steps

import (
	"log"
	"math/rand"
	"strings"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	"github.com/adam-0001/go-talin/modules/captcha"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
)

func HandleCheckpoint(t *shopifyTasks.ShopifyTask, alreadySolved bool) (bool, error) {
	if strings.Contains(t.CheckoutUrl, "/checkpoint") {
		if !alreadySolved {
			t.SetStatus("Checkpoint Found", colors.Yellow)
			log.Println("[SHOPIFY] Checkpoint found:", t.CheckoutUrl)
			cookies := t.Session.Client.Jar.Cookies(&t.ParsedUrl)
			toSend := map[string]string{}
			for _, cookie := range cookies {
				toSend[cookie.Name] = cookie.Value
			}
			t.SetStatus("Awaiting Captcha", colors.Yellow)
			newCookies, err := captcha.SolveShopifyCheckpoint(t, t.SiteUrl, toSend)
			if err != nil {
				return false, errs.ErrSolvingCheckpoint
			}
			log.Println("[SHOPIFY] Got new cookies:", newCookies)
			t.Session.ClearCookies()
			for name, value := range newCookies {
				t.Session.SetCookie(&t.ParsedUrl, name, value)
			}
			t.SetStatus("Processing Captcha", colors.Yellow)
		}
		t.SetStatus("Following Checkpoint Redirect", colors.Yellow)
		headers := []map[string]string{
			{"user-agent": utils.GetUserAgent()},
		}
		resp, err := t.Session.Get(t.SiteUrl+"/checkout", headers, nil)
		if err != nil {
			log.Println("[SHOPIFY] error with request (checkpoint):", err)
			log.Println("[SHOPIFY] Rotating Proxy")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			t.Session.SetProxy(newProxy)
			return true, errs.ErrorWithRequest
		}
		parseAuthTokenAndSiteKey(resp.Text, t)
		t.CheckoutUrl = resp.Url()
		t.LastResponse = resp
		return true, nil
	}
	return true, nil
}
