// logstashbrcvr project logstashbrcvr.go
package main

/*

The overall idea is to get Logstash downtime failures into NewRelic.
So this simple HTTP service receives heartbeat messages from Logstash [1] on the *private* HTTP endpoint
':8080/rcv' and "relays" these heartbeats to an Nginx proxy definition (see below). Nginx itself then exposes
that proxied endpoint to an *public* HTTP endpoint ':9090/mon' where New Relic (or something similar) can monitor
the availability of Logstash (or any service which provides heartbeats over HTTP).

Example Nginx proxy configuration:

	upstream logstashbrcv {
	    # the internal HTTP endpoint
	    server <private-ip>:8080;
	}

	# the proxy itself
	server {
	    listen <public-IP>:9090;

	    location / {
	        // Fail after 10 seconds without read from proxied service.
	        proxy_read_timeout 10s;
	        proxy_pass http://logstashhbrcv;
	    }

	}

                                                                        +-------------------------------+
                                                                        | NewRelic availability checker |
                                                                        +-----------------^-------------+
                                                                                          |
                                                                                          |
                                                                                          |
                                                                                          |
                                                                                          |
+--------------------+            +----------------------+             +------------------v-----------------+
|                    |            |                      |             |                                    |
| Logstash heartbeat |            | logstashbrcvr        |             | Nginx with publicly accessible URL |
| http output plugin +------------> HTTP "Relay" Service +-------------+ for NewRelic availability checker  |
|                    |            |                      |             | proxying logstashbrcvr             |
+--------------------+            +----------------------+             +------------------------------------+

(ASCI art created with http://asciiflow.com/)

[1] https://www.elastic.co/blog/how-to-check-logstashs-pulse

*/

import (
	"fmt"
	"log"
	"net/http"
)

// DEBUG LOGGING
const debug debugging = true // or flip to false
type debugging bool

func (d debugging) Printf(format string, args ...interface{}) {
	if d {
		log.Printf("  DEBUG  "+format, args...)
	}
}

type HeartbeatReceiver struct {
	heartbeatsChan *chan bool
}

type HeartbeatMonitor struct {
	heartbeatsChan *chan bool
}

// Implement http.Handler iface.
func (hbMon *HeartbeatMonitor) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	/*
		Listen to heartbeat messages on the channel in a blocking fashion.
		This blocking channel reading will result in an '504 Gateway Time-out' error on the Nginx proxy definition
		site, if no new heartbeat messages are available on the channel. If that's happening, it will cause
		NewRelic's availability checker to trigger an alert.
	*/
	for hb := range *hbMon.heartbeatsChan {
		// Reference: https://gobyexample.com/string-formatting
		debug.Printf("(HeartbeatMonitor.ServeHTTP)  >>  Served heartbeat from channel: '%t'", hb)
		fmt.Fprintf(w, "{\"status\": \"ok\"}\n")
		return
	}
}

// Implement http.Handler iface.
func (hbRcv *HeartbeatReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// The idea here is to only "forward" heartbeat messages coming from Logstash to the
	// New Relic facing URL to enable availability checking.
	select {
	case *hbRcv.heartbeatsChan <- true:
		debug.Printf("(HeartbeatReceiver.ServeHTTP) >>  Forwarded received heartbeat message.\n")
	// Make channel sending non-blocking to prevent errors on Logstash's site (which would be counterproductive).
	// "If the send cannot go through immediately the default case will be selected."
	// Reference: http://blog.golang.org/go-concurrency-patterns-timing-out-and
	default:
		debug.Printf("(HeartbeatReceiver.ServeHTTP) >>  Dropping heartbeat message (channel full).\n")
	}

}

func main() {

	// The buffered heartbeat exchange channel.
	heartbeatsChan := make(chan bool, 1)
	// Use the same channel in both HTTP endpoints.
	hbRcv := HeartbeatReceiver{
		heartbeatsChan: &heartbeatsChan,
	}
	hbMon := HeartbeatMonitor{
		heartbeatsChan: &heartbeatsChan,
	}
	// Define routes and corresponding handler functions.
	http.Handle("/rcv", &hbRcv)
	http.Handle("/mon", &hbMon)
	// Start the webserver providing the two defines endpoints.
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}

}
