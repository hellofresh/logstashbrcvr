logstashbrcvr (Logstash Heartbeat Receiver)
===========



The overall idea is to get [Logstash](https://www.elastic.co/products/logstash) downtime failures into New Relic.  

So this simple HTTP service receives [heartbeat messages from Logstash](https://www.elastic.co/blog/how-to-check-logstashs-pulse) on the **private** HTTP endpoint `:8080/rcv` and "relays" these heartbeats to an [Nginx](http://nginx.org/en/) proxy definition (see below). Nginx itself then exposes that proxied endpoint to an **public** HTTP endpoint `:9090/mon` where New Relic (or something similar) can monitor the availability of Logstash (or any service which provides heartbeats over HTTP).  
**Important: The assumption here is that the frequency of heartbeat HTTP messages comming in over the _private_ endpoint `:8080/rcv` is _much higher_ than the monitoring requests using the _public_ endpoint `:9090/mon`.** Otherwise the monitoring requests will drain the heartbeat channel too fast and causing false alerts in the availability checking service (New Relic).

**A graphical representation of the idea:**

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
          | http output plugin +------------> HTTP "Relay" Service <-------------+ for NewRelic availability checker  |
          |                    |            |                      |             | proxying logstashbrcvr             |
          +--------------------+            +----------------------+             +------------------------------------+

          (ASCI art created with http://asciiflow.com/)

**Example Nginx proxy configuration:**

          upstream logstashbrcv {
              # the internal HTTP endpoint
              // Prevent Denial-of-Service attacks which could be caused by the blocking channel read in logstashbrcvr.
              server <private-ip>:8080 max_conns=20;
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



