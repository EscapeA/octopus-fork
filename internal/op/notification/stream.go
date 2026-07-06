package notification

import (
	"sync"

	"github.com/lingyuins/octopus/internal/model"
)

const notificationSubscriberBuffer = 128

var (
	subscribersMu sync.RWMutex
	subscribers   = make(map[chan model.Notification]struct{})
)

func Subscribe() chan model.Notification {
	ch := make(chan model.Notification, notificationSubscriberBuffer)
	subscribersMu.Lock()
	subscribers[ch] = struct{}{}
	subscribersMu.Unlock()
	return ch
}

func Unsubscribe(ch chan model.Notification) {
	subscribersMu.Lock()
	if _, ok := subscribers[ch]; ok {
		delete(subscribers, ch)
		close(ch)
	}
	subscribersMu.Unlock()
}

func Publish(n model.Notification) {
	subscribersMu.RLock()
	defer subscribersMu.RUnlock()
	for ch := range subscribers {
		select {
		case ch <- n:
		default:
		}
	}
}
