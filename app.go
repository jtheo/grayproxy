package main

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/andviro/grayproxy/pkg/gelf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	inMessages = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "grayproxy",
		Name:      "in_total_messages",
		Help:      "The total number of incoming messages",
	})

	outMessages = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "grayproxy",
		Name:      "out_total_messages",
		Help:      "The total number of outcoming messages",
	})

	errorsMessages = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "grayproxy",
		Name:      "total_errors",
		Help:      "The total number of errors",
	})

	latency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "grayproxy",
		Name:      "out_latency",
		Help:      "The total number of errors",
	})
)

type listener interface {
	Listen(dest chan<- gelf.Chunk) (err error)
}

type sender interface {
	Send(data []byte) (err error)
}

type queue interface {
	Put(data []byte) error
	ReadChan() <-chan []byte
	Close() error
}

type app struct {
	inputURLs   urlList
	outputURLs  urlList
	verbose     bool
	sendTimeout int
	dataDir     string
	metricsOn   bool

	ins        []listener
	outs       []sender
	sendErrors []error
	q          queue
}

func (app *app) enqueue(msgs <-chan gelf.Chunk) {
	for msg := range msgs {
		inMessages.Inc()
		if err := app.q.Put(msg); err != nil {
			panic(err)
		}
	}
}

func (app *app) dequeue() {
	for msg := range app.q.ReadChan() {
		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			latency.Observe(v)
		}))

		// var sent bool
		if app.verbose {
			log.Println(string(msg))
		}

		counter := 0
		for i, out := range app.outs {
			err := out.Send(msg)
			if err != nil {
				if app.sendErrors[i] == nil {
					log.Printf("out %d: %v", i, err)
					errorsMessages.Inc()
					app.sendErrors[i] = err
				}
				continue
			}
			outMessages.Inc()
			if app.sendErrors[i] != nil {
				log.Printf("out %d is now alive", i)
				app.sendErrors[i] = nil
			}
			// sent = true
			// break
			counter++
		}
		if counter != len(app.outs) {
			// 	sent = true
			// }
			// if !sent {
			if app.dataDir == "" {
				continue
			}
			if err := app.q.Put(msg); err != nil {
				panic(err)
			}
		}
		timer.ObserveDuration()
		outMessages.Inc()
	}
}

func (app *app) run() (err error) {
	if err = app.configure(); err != nil {
		return
	}
	defer app.q.Close()
	msgs := make(chan gelf.Chunk, len(app.ins)*1000000)
	defer close(msgs)
	var wg sync.WaitGroup
	for i := range app.ins {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := app.ins[i].Listen(msgs)
			if err != nil {
				log.Printf("Input %d exited with error: %+v", i, err)
			}
		}(i)
	}
	go app.enqueue(msgs)
	go app.dequeue()
	log.Printf("starting grayproxy")

	if app.metricsOn {
		port := 9112
		promAddr := "0.0.0.0:" + strconv.Itoa(port)
		metricsPath := "/metrics"
		log.Printf("Starting metrics Listener on port %s%s\n", promAddr, metricsPath)
		http.Handle(metricsPath, promhttp.Handler())
		log.Fatal(http.ListenAndServe(promAddr, nil))
	}
	wg.Wait()
	return
}
