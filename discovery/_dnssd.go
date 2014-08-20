package discovery

import (
	"github.com/andrewtj/dnssd"
	"log"
)

func DnsRegisterService(name, serviceType string, port int) (*dnssd.RegisterOp, error) {
	callback := func(op *dnssd.RegisterOp, err error, add bool, name, serviceType, domain string) {
		if err != nil {
			// op is now inactive
			log.Printf("dnsRegisterService: Service registration failed: %s", err)
		}
		if add {
			log.Printf("dnsRegisterService: Service registered as “%s“ in %s", name, domain)
		} else {
			log.Printf("dnsRegisterService: Service “%s” removed from %s", name, domain)
		}
	}

	return dnssd.StartRegisterOp(name, serviceType, port, callback)
}
