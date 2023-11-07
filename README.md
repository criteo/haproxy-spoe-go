**Please note:** This repository is currently unmaintained. Please switch to project https://github.com/negasus/haproxy-spoe-go if you want to use a golang version of haproxy-spoe library.

[![GoDoc](https://godoc.org/github.com/criteo/haproxy-spoe-go?status.svg)](https://godoc.org/github.com/criteo/haproxy-spoe-go)

# SPOE in Go

An implementation of the SPOP protocol in Go. (https://www.haproxy.org/download/2.0/doc/SPOE.txt)

## SPOE

From Haproxy's documentation :

> SPOE is a feature introduced in HAProxy 1.7. It makes possible the
> communication with external components to retrieve some info. The idea started
> with the problems caused by most ldap libs not working fine in event-driven
> systems (often at least the connect() is blocking). So, it is hard to properly
> implement Single Sign On solution (SSO) in HAProxy. The SPOE will ease this
> kind of processing, or we hope so.
>
> Now, the aim of SPOE is to allow any kind of offloading on the streams. First
> releases, besides being experimental, won't do lot of things. As we will see,
> there are few handled events and even less actions supported. Actually, for
> now, the SPOE can offload the processing before "tcp-request content",
> "tcp-response content", "http-request" and "http-response" rules. And it only
> supports variables definition. But, in spite of these limited features, we can
> easily imagine to implement SSO solution, ip reputation or ip geolocation
> services.

## How to use

```golang
package main

import (
	"fmt"
	"net"

	"log"

	spoe "github.com/criteo/haproxy-spoe-go"
)

func getReputation(ip net.IP) (float64, error) {
	// implement IP reputation code here
	return 1.0, nil
}

func main() {
	agent := spoe.New(func(messages *spoe.MessageIterator) ([]spoe.Action, error) {
		reputation := 0.0

		for messages.Next() {
			msg := messages.Message

			if msg.Name != "ip-rep" {
				continue
			}

			var ip net.IP
			for msg.Args.Next() {
				arg := msg.Args.Arg

				if arg.Name == "ip" {
					var ok bool
					ip, ok = arg.Value.(net.IP)
					if !ok {
						return nil, fmt.Errorf("spoe handler: expected ip in message, got %+v", ip)
					}
				}
			}

			var err error
			reputation, err = getReputation(ip)
			if err != nil {
				return nil, fmt.Errorf("spoe handler: error processing request: %s", err)
			}
		}

		return []spoe.Action{
			spoe.ActionSetVar{
				Name:  "reputation",
				Scope: spoe.VarScopeSession,
				Value: reputation,
			},
		}, nil
	})

	if err := agent.ListenAndServe(":9000"); err != nil {
		log.Fatal(err)
	}
}


```
