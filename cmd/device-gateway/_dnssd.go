package main

import (
	"fmt"
	"github.com/andrewtj/dnssd"
	"log"
)

func dnsRegisterService(conf *Config) (*dnssd.RegisterOp, error) {
	var (
		restConf Protocol
		ok       bool
	)
	if restConf, ok = conf.Protocols[ProtocolTypeREST]; !ok {
		return nil, fmt.Errorf("dnsRegisterService: Missing configuration section for protocol %s", ProtocolTypeREST)
	}

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

	return dnssd.StartProxyRegisterOp(conf.Name, "_patchwork-dgw._tcp", conf.Addr, restConf.Port, callback)
}
