package safe

import (
	"errors"
	"log"
	"time"

	"github.com/adam-0001/go-talin/colors"
	errs "github.com/adam-0001/go-talin/errors"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"

	"github.com/adam-0001/go-talin/modules/shopify/monitors"
	"github.com/adam-0001/go-talin/modules/shopify/steps"
)

func SafeCheckout(t *shopifyTasks.ShopifyTask) {
	// t.Profile.Payment.CardNumber = "4242424242424242"
	monType := monitors.CheckMonitorType(t)
	var err error
	t.StartTime = time.Now()
	// t.Session.Get(t.SiteUrl+"/robots.txt", nil, nil)
	if t.ForceLogin {
		x := true
		captcha := false
		for x {
			if !t.IsRunning() {
				return
			}
			t.SetStatus("Logging In", colors.Yellow)
			err = steps.LoginToAccountBeforeStart(t, captcha)
			switch err {
			case nil:
				x = false
			case errs.ErrNoAccountSpecified:
				log.Println("[Shopify] No account specified, returning")
				t.End("Stopped - No Account Specified")
				return
			case errs.ErrCaptchaRequired:
				log.Println("[Shopify] Captcha required, retrying")
				time.Sleep(t.Settings.Shopify.ErrorDelay)
				captcha = true
			case errs.ErrInvalidAccountInfo:
				t.AccountList = t.AccountList[1:]
				log.Println("[Shopify] Invalid account info, retrying")
				errs.HandleErrorStatus(t, err)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
			case errs.ErrorWithRequest:
				errs.HandleErrorStatus(t, err)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
			default:
				log.Println("[Shopify] Monitor err (login):", err)
				t.SetStatus("Retrying Login", colors.Red)
				time.Sleep(t.Settings.Shopify.ErrorDelay)
			}
		}
	}
	if monType != "var" {
		t.SetStatus("Monitoring Product", colors.Yellow)
	}
	err = func() error {
		for {
			if !t.IsRunning() {
				return errs.ErrorTaskStop
			}
			switch monType {
			case "kws":
				t.Product, err = monitors.KeywordMonitor(t)

			case "link":
				t.Product, err = monitors.LinkMonitor(t)
			case "var":
				return nil
			default:
				t.End("Stopped - Monitor Type Not Found")
				return errors.New("Unknown monitor type")
			}
			switch err {
			case nil:
				return nil
			case errs.ErrorTaskStop:
				return err
			case errs.ErrOutOfStock:
				if t.ReleaseMode {
					errs.HandleErrorStatus(t, err)
					time.Sleep(t.Settings.Shopify.MonitorDelay)

				} else {
					return nil
				}
			case errs.ErrInvalidParse:
				t.End("Stopped - Invalid Parse Method")
				return errors.New("Invalid Parse Method")
			case errs.ErrSizeNotFound:
				errs.HandleErrorStatus(t, err)
				time.Sleep(t.Settings.Shopify.MonitorDelay)

			default:
				t.SetStatus("Error Monitoring - Retrying", colors.Red)
				log.Println("Unexpected error (Monitor):", err.Error())
				time.Sleep(t.Settings.Shopify.MonitorDelay)
			}
		}
	}()
	if err != nil {
		return
	}
	log.Println("[Shopify] Adding to Cart - Product:", t.Product)
	t.SetProductName(t.Product.Title)
	t.SetStatus("Adding to Cart", colors.Yellow)
	for {
		if !t.IsRunning() {
			return
		}
		if monType == "var" {
			err = steps.Atc(t, t.Variant)
		} else {
			err = steps.Atc(t, t.Product.Variant)
		}
		if err != nil {
			if err == errs.ErrorWithRequest {
				errs.HandleErrorStatus(t, err)
			} else {
				t.SetStatus("Retrying ATC", colors.Yellow)
			}
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		} else {
			break
		}
	}
	for {
		if !t.IsRunning() {
			return
		}
		err = steps.CreateCheckoutSession(t)
		if err == nil {
			break
		} else if err == steps.ErrUnsupportedStore {
			t.End("Stopped - Unsupported Store")
			return
		} else {
			errs.HandleErrorStatus(t, err)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		}
	}
	var tmp = false
	for {
		if !t.IsRunning() {
			return
		}

		tmp, err = steps.HandleCheckpoint(t, false)
		if err == nil {

			break
		}
		if err != nil && tmp {
			errs.HandleErrorStatus(t, err)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		} else if err != nil && !tmp {
			errs.HandleErrorStatus(t, err)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		}
	}
	err = steps.HandleQueue(t)
	if err != nil {
		// switch err {
		// case errs.ErrorTaskStop:
		// 	return

		// }
		if errs.ErrorTaskStop == err {
			return
		}
		log.Println("[Shopify] Error handling queue:", err)
	}
	captcha := false
	for {
		if !t.IsRunning() {
			return
		}

		err = steps.HandleAccount(t, captcha)
		if err == nil {
			break
		}
		switch err {
		case errs.ErrNoAccountSpecified:
			log.Println("[Shopify] No account specified, returning")
			t.End("Stopped - No Account Specified")
			return
		case errs.ErrCaptchaRequired:
			log.Println("[Shopify] Captcha required, retrying")
			time.Sleep(t.Settings.Shopify.ErrorDelay)
			captcha = true
		case errs.ErrInvalidAccountInfo:
			t.AccountList = t.AccountList[1:]
			log.Println("[Shopify] Invalid account info, retrying")
			errs.HandleErrorStatus(t, err)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		case errs.ErrorWithRequest:
			errs.HandleErrorStatus(t, err)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		default:
			log.Println("[Shopify] Monitor err (login):", err)
			t.SetStatus("Retrying Login", colors.Red)
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		}
	}
	t.SetStatus("Settting Delivery Info", colors.Yellow)
	tmp = false
	for {
		err = steps.SetDeliveryInfo(t, tmp)
		if err == nil {
			break
		}
		tmp = true
		errs.HandleErrorStatus(t, err)
		t.SetStatus("Retrying Delivery Info", colors.Red)
		time.Sleep(t.Settings.Shopify.ErrorDelay)
	}
	err = steps.SetShippingRate(t)
	if err != nil {
		return //Can assume this is a stop task b/c it is the only possible return val
	}
	for {
		err = steps.FetchPaymentGateway(t)
		if err != nil {
			switch err {
			case errs.ErrorTaskStop:
				return
			case errs.ErrorFatalStopTask:
				t.End("Stopped - Payment Method Not Accepted")
				return
			case errs.ErrorWithRequest:
				errs.HandleErrorStatus(t, err)
			}
			time.Sleep(t.Settings.Shopify.ErrorDelay)
		} else {
			break
		}
	}
	log.Println("Submitted Checkout In:", time.Since(t.StartTime))
	err = steps.SubmitPayment(t)
	if err != nil {
		return
	}
	t.EndSuccessfulCheckout()

}
