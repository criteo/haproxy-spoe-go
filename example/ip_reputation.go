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
