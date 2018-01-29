package k8s

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubewatch "k8s.io/apimachinery/pkg/watch"
	kubeclient "k8s.io/client-go/kubernetes"
	kubev1core "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// Number of object pointers. Big enough so it won't be hit anytime soon with resonable GetNewEvents frequency.
	localEventsBufferSize = 100000
)

var (
	// Last time of event since unix epoch in seconds
	lastEventTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "launcher",
			Subsystem: "events",
			Name:      "last_time_seconds",
			Help:      "Last time of event since unix epoch in seconds.",
		})
	totalEventsNum = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "launcher",
			Subsystem: "events",
			Name:      "total",
			Help:      "The total number of events.",
		})
	scrapEventsDuration = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: "launcher",
			Subsystem: "events",
			Name:      "scrape_duration_milliseconds",
			Help:      "Time spent scraping events in milliseconds.",
		})
)

func init() {
	prometheus.MustRegister(lastEventTimestamp)
	prometheus.MustRegister(totalEventsNum)
	prometheus.MustRegister(scrapEventsDuration)
}

// EventSource produces kubernetes events.
type EventSource struct {
	// Large local buffer, periodically read.
	localEventsBuffer chan *apiv1.Event
	eventClient       kubev1core.EventInterface
}

// GetNewEvents returns the kubernetes events that have been fired since the
// previous invocation of the function.
func (source *EventSource) GetNewEvents() []*apiv1.Event {
	startTime := time.Now()
	defer func() {
		lastEventTimestamp.Set(float64(time.Now().Unix()))
		scrapEventsDuration.Observe(float64(time.Since(startTime)) / float64(time.Millisecond))
	}()

	// Get all data from the buffer.
	events := []*apiv1.Event{}
event_loop:
	for {
		select {
		case event := <-source.localEventsBuffer:
			events = append(events, event)
		default:
			break event_loop
		}
	}

	totalEventsNum.Add(float64(len(events)))

	return events
}

func (source *EventSource) setupWatcher() (<-chan kubewatch.Event, error) {
	// Do not write old events.
	events, err := source.eventClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	watcher, err := source.eventClient.Watch(
		metav1.ListOptions{
			Watch:           true,
			ResourceVersion: events.ResourceVersion,
		},
	)
	if err != nil {
		return nil, err
	}

	return watcher.ResultChan(), nil
}

func (source *EventSource) watch(cancel <-chan interface{}) {
	// Outer loop, for reconnections.
	for {

		// Setup watcher on events
		watchChannel, err := source.setupWatcher()
		if err != nil {
			log.Errorf("failed to setup watch for events: %v", err)
			time.Sleep(time.Second)
			continue
		}

		// Inner loop, for update processing.
	inner_loop:
		for {
			select {
			case watchUpdate, ok := <-watchChannel:
				if !ok {
					log.Errorf("Event watch channel closed")
					break inner_loop
				}

				if watchUpdate.Type == kubewatch.Error {
					if status, ok := watchUpdate.Object.(*metav1.Status); ok {
						log.Errorf("Error during watch: %#v", status)
						break inner_loop
					}
					log.Errorf("Received unexpected error: %#v", watchUpdate.Object)
					break inner_loop
				}

				if event, ok := watchUpdate.Object.(*apiv1.Event); ok {
					switch watchUpdate.Type {
					case kubewatch.Added, kubewatch.Modified:
						select {
						case source.localEventsBuffer <- event:
							// Ok, buffer not full.
						default:
							// Buffer full, need to drop the event.
							log.Errorf("Event buffer full, dropping event")
						}
					case kubewatch.Deleted:
						// Deleted events are silently ignored.
					default:
						log.Warningf("Unknown watchUpdate.Type: %#v", watchUpdate.Type)
					}
				} else {
					log.Errorf("Wrong object received: %v", watchUpdate)
				}

			case <-cancel:
				log.Infof("Event watching stopped")
				return
			}
		}
	}
}

// NewEventSource listens to kuberentes events in namespace. Call GetNewEvents
// periodically to retrieve batches of events.
func NewEventSource(client *kubeclient.Clientset, namespace string) *EventSource {
	eventClient := client.CoreV1().Events(namespace)
	result := EventSource{
		localEventsBuffer: make(chan *apiv1.Event, localEventsBufferSize),
		eventClient:       eventClient,
	}
	return &result
}

// Start starts watching for Kubernetes event. The cancel channel can be used to
// terminate the operation.
func (source *EventSource) Start(cancel <-chan interface{}) {
	source.watch(cancel)
}
