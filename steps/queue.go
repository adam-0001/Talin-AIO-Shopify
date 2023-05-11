package steps

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
	"github.com/adam-0001/go-talin/utils"
	"github.com/adam-0001/requests"
)

func HandleQueue(t *shopifyTasks.ShopifyTask) error {
	if strings.Contains(t.CheckoutUrl, "/throttle/queue") {
		var (
			// urlRes     string
			queueToken string
			queueState string = "PollContinue"
		)
		t.SetStatus("Waiting in Queue", colors.Yellow)
		log.Println("[Shopify] Queue found - handling (" + t.CheckoutUrl + ")")
		cookies := t.Session.Client.Jar.Cookies(&t.ParsedUrl)
		for item := range cookies {
			if cookies[item].Name == "_checkout_queue_token" {
				queueToken = cookies[item].Value
			}
		}

		if queueToken != "" {
			pollAfter := time.Until(time.Now())
			for queueState == "PollContinue" {
				time.Sleep(pollAfter)
				if !t.IsRunning() {
					return errs.ErrorTaskStop
				}
				headers := []map[string]string{
					{"user-agent": utils.GetUserAgent()},
					{"content-type": "application/json"},
				}
				payload := fmt.Sprintf(`{"query": "\n      {\n        poll(token: $token) {\n          token\n          pollAfter\n          queueEtaSeconds\n          productVariantAvailability {\n            id\n            available\n          }\n        }\n      }\n    ", "variables": {
						"token": %v}}`, queueToken)

				resp, err := t.Session.Post(t.SiteUrl+"/queue/poll", headers, payload)
				if err != nil {
					log.Println("[SHOPIFY] error with request (queue):", err)
					log.Println("[SHOPIFY] Rotating Proxy")
					newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
					errs.HandleErrorStatus(t, errs.ErrorWithRequest)
					t.Session.SetProxy(newProxy)
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					time.Sleep(t.Settings.Shopify.ErrorDelay)
					continue
				}
				if resp.StatusCode() != 200 {
					log.Println("[SHOPIFY] unexpected status code:", resp.StatusCode())
					t.SetStatus("Bad Status: "+string(rune(resp.StatusCode())), colors.Red)
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					time.Sleep(t.Settings.Shopify.ErrorDelay)
					continue
				}
				pollRes := pollResponse{}
				err = resp.Json(&pollRes)
				if err != nil {
					t.SetStatus("Error Decoding JSON", colors.Red)
					log.Println("[SHOPIFY] error with json (queue):", err)
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					time.Sleep(t.Settings.Shopify.ErrorDelay)
					continue
				}
				queueState = pollRes.Data.Poll.Typename
				queueToken = pollRes.Data.Poll.Token
				if queueState == "PollComplete" {
					log.Println("[SHOPIFY] Queue resolved")
					// t.SetStatus("Queue resolved", color)
					break
				}
				if tmp := pollRes.Data.Poll.QueueEtaSeconds; tmp > 0 {
					if tmp >= 60 {
						eta := tmp / 60
						t.SetStatus("Waiting In Queue ("+fmt.Sprint(eta)+"m)", colors.Yellow)
					} else {
						t.SetStatus("Waiting In Queue ("+fmt.Sprint(tmp)+"s)", colors.Yellow)
					}
					log.Println("[SHOPIFY] Queue ETA:", pollRes.Data.Poll.QueueEtaSeconds, "seconds")
				}
				pollAfter = time.Until(pollRes.Data.Poll.PollAfter)
			}
			t.Session.SetCookie(&t.ParsedUrl, "_checkout_queue_token", queueToken)
			cookies = t.Session.Client.Jar.Cookies(&t.ParsedUrl)
			var (
				replayData string
				method     string
				ur         string
				replayObj  queueReplayData
				resp       requests.Response
				err        error
			)
			for item := range cookies {
				if cookies[item].Name == "_queue_replay_data" {
					replayData = cookies[item].Value
					break
				}
			}
			if replayData == "" {
				log.Println("[SHOPIFY] No replay data found!")
				errs.HandleErrorStatus(t, errs.ErrorWithRequest)

			} else {
				new, err := base64.StdEncoding.DecodeString(replayData)
				if err != nil {
					log.Println("[SHOPIFY] Error decoding replay data:", err)
					// return errs.ErrorFatalStopTask
				}
				err = json.Unmarshal(new, &replayObj)
				if err != nil {
					log.Println("[SHOPIFY] Error parsing replay data:", err)
					// return errs.ErrorFatalStopTask
				}
				ur = strings.ReplaceAll(ur, "\\", "")
			}
			if method == "GET" {
				for {
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					headers := []map[string]string{
						{"user-agent": utils.GetUserAgent()},
					}
					resp, err = t.Session.Get(ur, headers, nil)
					if err != nil {
						log.Println("[SHOPIFY] error with request (queue):", err)
						log.Println("[SHOPIFY] Rotating Proxy")
						newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
						errs.HandleErrorStatus(t, errs.ErrorWithRequest)
						t.Session.SetProxy(newProxy)
						if !t.IsRunning() {
							return errs.ErrorTaskStop
						}
						time.Sleep(t.Settings.Shopify.ErrorDelay)
						continue
					}
					break
				}
			} else if method == "PUT" || method == "POST" || method == "PATCH" {
				body := fmt.Sprintf(`updates[]=%v&attributes[checkout_clicked]=%v&checkout=%v`, replayObj.Params.Updates[0], replayObj.Params.Attributes.CheckoutClicked, replayObj.Params.Checkout)
				headers := []map[string]string{
					{"user-agent": utils.GetUserAgent()},
					{"content-type": "application/x-www-form-urlencoded"},
					{"referer": t.SiteUrl + "/throttle/queue"},
				}
				for {
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					resp, err = t.Session.MakeRequest(method, ur, headers, body)
					if err != nil {
						log.Println("[SHOPIFY] error with request (queue):", err)
						log.Println("[SHOPIFY] Rotating Proxy")
						newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
						errs.HandleErrorStatus(t, errs.ErrorWithRequest)
						t.Session.SetProxy(newProxy)
						if !t.IsRunning() {

							return errs.ErrorTaskStop
						}
						time.Sleep(t.Settings.Shopify.ErrorDelay)
						continue
					}
					break
				}
			} else {
				for {
					if !t.IsRunning() {
						return errs.ErrorTaskStop
					}
					headers := []map[string]string{
						{"user-agent": utils.GetUserAgent()},
					}
					resp, err = t.Session.Get(t.SiteUrl+"/checkout", headers, nil)
					if err != nil {
						log.Println("[SHOPIFY] error with request (queue):", err)
						log.Println("[SHOPIFY] Rotating Proxy")
						newProxy := t.CheckoutProxies[rand.Intn(len(t.CheckoutProxies))]
						t.Session.SetProxy(newProxy)
						errs.HandleErrorStatus(t, errs.ErrorWithRequest)
						if !t.IsRunning() {

							return errs.ErrorTaskStop
						}
						time.Sleep(t.Settings.Shopify.ErrorDelay)
						continue
					}
					break
				}
			}
			t.CheckoutUrl = resp.Url()
			t.LastResponse = resp
			parseAuthTokenAndSiteKey(resp.Text, t)
		} else {
			log.Println("[SHOPIFY] No checkout queue found (possible old queue)")
		}

	}
	return nil
}

//https://go.dev/play/p/QGOCvYgq49K
type pollResponse struct {
	Data struct {
		Poll struct {
			Token                      string    `json:"token"`
			PollAfter                  time.Time `json:"pollAfter"`
			QueueEtaSeconds            int       `json:"queueEtaSeconds"`
			ProductVariantAvailability []struct {
				ID        string `json:"id"`
				Available bool   `json:"available"`
			} `json:"productVariantAvailability"`
			Typename string `json:"__typename"`
		} `json:"poll"`
	} `json:"data"`
}

type queueReplayData struct {
	URL    string `json:"url"`
	Method string `json:"method"`
	Params struct {
		Updates    []string `json:"updates"`
		Attributes struct {
			CheckoutClicked string `json:"checkout_clicked"`
		} `json:"attributes"`
		Checkout string `json:"checkout"`
	} `json:"params"`
}
