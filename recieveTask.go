package shopify

import (
	"fmt"
	"strings"

	"github.com/adam-0001/go-talin/modules/shopify/fast"
	"github.com/adam-0001/go-talin/modules/shopify/preload"

	"github.com/adam-0001/go-talin/modules/shopify/safe"
	shopifyTasks "github.com/adam-0001/go-talin/tasks/shopify"
)

func StartTask(task *shopifyTasks.ShopifyTask) error {
	// task.Status = "Started"
	// task.StartedAt = time.Now()
	switch strings.ToLower(task.Mode) {
	case "safe":
		go safe.SafeCheckout(task)
	case "preload":
		go preload.PreloadCheckout(task)
	case "fast":
		go fast.FastCheckout(task)
	default:
		return fmt.Errorf("unknown shopify mode: %s", task.Mode)
	}
	return nil

}
