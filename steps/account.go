package steps

import (
	"errors"
	"log"
	"math/rand"
	"net/url"
	"strings"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	"github.com/adam-0001/go-talin/modules/captcha"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
)

func HandleAccount(t *shopifyTasks.ShopifyTask, retry bool) error {
	if strings.Contains(t.CheckoutUrl, "account/login") {
		log.Println("[SHOPIFY] Login Required")
		t.SetStatus("Login Required", colors.Red)
		if len(t.AccountList) == 0 {
			log.Println("[SHOPIFY] No accounts specified - returning")
			return errs.ErrNoAccountSpecified
		}
		t.SetStatus("Attempting Login", colors.Yellow)
		chosenAccount := t.AccountList[0]
		log.Printf("[SHOPIFY] Chosen Account: %s", chosenAccount.Username)
		var err error
		checkoutUrl, err := url.QueryUnescape(strings.Split(strings.Split(t.CheckoutUrl, "checkout_url=")[1], "&")[0])
		if err != nil {
			log.Println(`[SHOPIFY] Error unescaping "`+t.CheckoutUrl+`":`, err)
			return err
		}
		t.CheckoutUrl = checkoutUrl
		headers := []map[string]string{
			{"user-agent": utils.GetUserAgent()},
			{"content-type": "application/x-www-form-urlencoded"},
		}
		var v3Token string
		if retry {
			v3Token, err = captcha.SolveRecaptchaV3(t, t.Sitekey, "customer_login", t.SiteUrl)
			if err != nil {
				log.Println("[SHOPIFY] Error solving captcha:", err)
				return err
			}
		} else {
			v3Token = ""
		}
		payload := url.Values{}
		payload.Set("form_type", "customer_login")
		payload.Set("customer[email]", chosenAccount.Username)
		payload.Set("customer[password]", chosenAccount.Password)
		payload.Set("checkout_url", t.CheckoutUrl)
		payload.Set("checkout_url", t.CheckoutUrl)
		payload.Set("recaptcha-v3-token", v3Token)
		resp, err := t.Session.Post(t.SiteUrl+"/account/login", headers, payload.Encode())
		if err != nil {
			log.Println("[SHOPIFY] error with request (account):", err)
			log.Println("[SHOPIFY] Rotating Proxy")
			newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
			t.Session.SetProxy(newProxy)
			return errs.ErrorWithRequest
		}
		if strings.Contains(resp.Url(), "account/login") {
			log.Printf("[SHOPIFY] Login Failed (%v)\n", resp.StatusCode())
			return errors.New("failed to login") //invalid info
		}
		if strings.Contains(resp.Url(), "/challenge") {
			log.Println("[SHOPIFY] Captcha Required: ", resp.Url())
			return errs.ErrCaptchaRequired
		}
		t.CheckoutUrl = resp.Url()
	}
	return nil
}

func LoginToAccountBeforeStart(t *shopifyTasks.ShopifyTask, retry bool) error {
	headers := []map[string]string{
		{"user-agent": utils.GetUserAgent()},
		{"content-type": "application/x-www-form-urlencoded"},
	}
	if len(t.AccountList) == 0 {
		log.Println("[SHOPIFY] No accounts specified - returning")
		return errs.ErrNoAccountSpecified
	}
	chosenAccount := t.AccountList[rand.Intn(len(t.AccountList))]
	log.Printf("[SHOPIFY] Chosen Account: %s", chosenAccount.Username)
	resp, err := t.Session.Get(t.SiteUrl+"/login", headers, nil)
	if err != nil {
		log.Println("[SHOPIFY] error with request (account):", err)
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		return errs.ErrorWithRequest
	}
	var v3Token string
	if retry {
		v3Token, err = captcha.SolveRecaptchaV3(t, t.Sitekey, "customer_login", t.SiteUrl)
		if err != nil {
			log.Println("[SHOPIFY] Error solving captcha:", err)
			return err
		}
	} else {
		v3Token = ""
	}
	payload := url.Values{}
	payload.Set("form_type", "customer_login")
	payload.Set("customer[email]", chosenAccount.Username)
	payload.Set("customer[password]", chosenAccount.Password)
	payload.Set("checkout_url", t.CheckoutUrl)
	payload.Set("checkout_url", t.CheckoutUrl)
	payload.Set("recaptcha-v3-token", v3Token)
	resp, err = t.Session.Post(t.SiteUrl+"/account/login", headers, payload.Encode())
	if err != nil {
		log.Println("[SHOPIFY] error with request (account):", err)
		log.Println("[SHOPIFY] Rotating Proxy")
		newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
		t.Session.SetProxy(newProxy)
		return errs.ErrorWithRequest
	}
	if strings.Contains(resp.Url(), "account/login") {
		log.Printf("[SHOPIFY] Login Failed (%v)\n", resp.StatusCode())
		return errs.ErrInvalidAccountInfo
	}
	if strings.Contains(resp.Url(), "/challenge") {
		log.Println("[SHOPIFY] Captcha Required: ", resp.Url())
		if retry {
			return LoginToAccountBeforeStart(t, true)
		}
		return errs.ErrCaptchaRequired
	}
	return nil
}
