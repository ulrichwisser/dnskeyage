package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/miekg/dns"
)

const (
	resolvtimeout time.Duration = 5 * time.Second
	edns0size     uint16        = 4096
)

func getNewKeys(config *Configuration, zone string) []*dns.DNSKEY {
	for _, resolver := range config.Resolvers {
		msg, err := resolving(net.JoinHostPort(resolver, fmt.Sprintf("%d", config.Port)), false, zone, dns.TypeDNSKEY)
		if err != nil {
			log.Println(err)
			continue
		}
		if msg == nil {
			log.Printf("No answer from resolver %s", resolver)
			continue
		}
		result := make([]*dns.DNSKEY, 0)
		for _, rr := range msg.Answer {
			if rr.Header().Rrtype == dns.TypeDNSKEY {
				result = append(result, rr.(*dns.DNSKEY))
			}
		}
		return result
	}
	return nil
}

func resolving(server string, udp bool, qname string, qtype uint16) (*dns.Msg, error) {
	// Setting up query
	query := new(dns.Msg)
	query.SetQuestion(qname, qtype)
	query.SetEdns0(edns0size, false)
	query.IsEdns0().SetDo()
	query.RecursionDesired = true

	// Setting up resolver
	client := new(dns.Client)
	client.ReadTimeout = resolvtimeout
	if !udp {
		client.Net = "tcp"
	}

	// make the query and wait for answer
	r, _, err := client.Exchange(query, server)

	// check for errors
	if err != nil {
		if err == dns.ErrTruncated {
			return resolving(server, false, qname, qtype)
		}
		return nil, fmt.Errorf("resolving: error resolving %s (server %s), %s", qname, server, err)
	}
	if r == nil {
		return nil, fmt.Errorf("resolving: no answer resolving %s (server %s)", qname, server)
	}
	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("resolving: could not resolve %s rcode %s  (server %s)", qname, dns.RcodeToString[r.Rcode], server)
	}

	return r, nil
}
