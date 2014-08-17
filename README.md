# Patchwork Tookit - Lightweight Platform for the Network of Things (written in Golang)

Patchwork is a toolkit for connecting various devices into a network of things or, in a more broad case - Internet of Things (IoT). The main goal of creating this toolkit is having a lightweight set of components that can help to quickly integrate different devices (i.e. Arduinos, RaspberryPI’s, Plugwise, etc) into a smart environment and expose specific devices’ capabilities as RESTful/SOAP/CoAP/MQTT/etc services.

You can read more about the toolkit on the [website](http://patchwork-toolkit.github.io/).

## Quick start with binary distribution

TBD...

## Getting started with sources

The toolkit requires the latest stable Go release. If you don't have the Go installed read the official [Go installation guide](http://golang.org/doc/install).

Create a folder for your installation and change to it (all subsequent commands should be executed from this folder):

```
mkdir patchwork
```

Install dependencies:

```
GOPATH=`pwd` go get git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git
GOPATH=`pwd` go get github.com/andrewtj/dnssd
GOPATH=`pwd` go get github.com/bmizerany/pat
```

Checkout the sources from Github treating the current folder as a GOPATH directory:

```
GOPATH=`pwd` go get -d github.com/patchwork-toolkit/patchwork
```

Build the main Patchwork components (device gateway, devices catalog and services catalog binaries):

```
GOPATH=`pwd` go install github.com/patchwork-toolkit/patchwork/cmd/device-gateway
GOPATH=`pwd` go install github.com/patchwork-toolkit/patchwork/cmd/device-catalog
GOPATH=`pwd` go install github.com/patchwork-toolkit/patchwork/cmd/service-catalog
```

As a result the corresponding binaries will be created in the `bin` folder in the project folder:

```
bin/
bin/device-catalog
bin/device-gateway
bin/service-catalog
```

## What's next?

For the configuration and running guide see ["Getting Started" section of the website](http://patchwork-toolkit.github.io/getting-started/).

## Documentation

Documention is available in the ["Documentation" section of the website](http://patchwork-toolkit.github.io/docs/).




