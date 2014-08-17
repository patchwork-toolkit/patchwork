# Patchwork Tookit - Lightweight Platform for the Network of Things (written in Golang)

Patchwork is a toolkit for connecting various devices into a network of things or, in a more broad case - Internet of Things (IoT). The main goal of creating this toolkit is having a lightweight set of components that can help to quickly integrate different devices (i.e. Arduinos, RaspberryPI’s, Plugwise, etc) into a smart environment and expose specific devices’ capabilities as RESTful/SOAP/CoAP/MQTT/etc services.

You can read more about the toolkit on the [website](http://patchwork-toolkit.github.io/).

## Quick start with binary distribution

TBD...

## Getting started with sources

Create a folder for your installation and change to it (all subsequent commands should be executed from this folder):

```
mkdir patchwork
```

Checkout the sources from Github treating the current folder as a GOPATH directory:

```
GOPATH=`pwd` go get -d github.com/patchwork-toolkit/patchwork
```



Current version of the library requires a latest stable Go release. If you don't have the Go compiler installed, read the official [Go install guide](http://golang.org/doc/install).

Use go tool to install the package in your packages tree:

```
go get github.com/trustmaster/goflow
```

Then you can use it in import section of your Go programs:

```
go import "github.com/trustmaster/goflow"
```

## Basic Example

Below there is a listing of a simple program running a network of two processes.

![Greeter example diagram](http://flowbased.wdfiles.com/local--files/goflow/goflow-hello.png)

This first one generates greetings for given names, the second one prints them on screen. It demonstrates how components and graphs are defined and how they are embedded into the main program.

```
go
package main

import (
    "fmt"
    "github.com/trustmaster/goflow"
)

// A component that generates greetings
type Greeter struct {
    flow.Component               // component "superclass" embedded
    Name           <-chan string // input port
    Res            chan<- string // output port
}

// Reaction to a new name input
func (g *Greeter) OnName(name string) {
    greeting := fmt.Sprintf("Hello, %s!", name)
    // send the greeting to the output port
    g.Res <- greeting
}

// A component that prints its input on screen
type Printer struct {
    flow.Component
    Line <-chan string // inport
}

// Prints a line when it gets it
func (p *Printer) OnLine(line string) {
    fmt.Println(line)
}

// Our greeting network
type GreetingApp struct {
    flow.Graph               // graph "superclass" embedded
}

// Graph constructor and structure definition
func NewGreetingApp() *GreetingApp {
    n := new(GreetingApp) // creates the object in heap
    n.InitGraphState()    // allocates memory for the graph
    // Add processes to the network
    n.Add(new(Greeter), "greeter")
    n.Add(new(Printer), "printer")
    // Connect them with a channel
    n.Connect("greeter", "Res", "printer", "Line")
    // Our net has 1 inport mapped to greeter.Name
    n.MapInPort("In", "greeter", "Name")
    return n
}

func main() {
    // Create the network
    net := NewGreetingApp()
    // We need a channel to talk to it
    in := make(chan string)
    net.SetInPort("In", in)
    // Run the net
    flow.RunNet(net)
    // Now we can send some names and see what happens
    in <- "John"
    in <- "Boris"
    in <- "Hanna"
    // Close the input to shut the network down
    close(in)
    // Wait until the app has done its job
    <-net.Wait()
}
```

Looks a bit heavy for such a simple task but FBP is aimed at a bit more complex things than just printing on screen. So in more complex an realistic examples the infractructure pays the price.

You probably have one question left even after reading the comments in code: why do we need to wait for the finish signal? This is because flow-based world is asynchronous and while you expect things to happen in the same sequence as they are in main(), during runtime they don't necessarily follow the same order and the application might terminate before the network has done its job. To avoid this confusion we listen for a signal on network's `Wait()` channel which is closed when the network finishes its job.

## Terminology

Here are some Flow-based programming terms used in GoFlow:

* Component - the basic element that processes data. Its structure consists of input and output ports and state fields. Its behavior is the set of event handlers. In OOP terms Component is a Class.
* Connection - a link between 2 ports in the graph. In Go it is a channel of specific type.
* Graph - components and connections between them, forming a higher level entity. Graphs may represent composite components or entire applications. In OOP terms Graph is a Class.
* Network - is a Graph instance running in memory. In OOP terms a Network is an object of Graph class.
* Port - is a property of a Component or Graph through which it communicates with the outer world. There are input ports (Inports) and output ports (Outports). For GoFlow components it is a channel field.
* Process - is a Component instance running in memory. In OOP terms a Process is an object of Component class.

More terms can be found in [flowbased terms](http://flowbased.org/terms) and [FBP wiki](http://www.jpaulmorrison.com/cgi-bin/wiki.pl?action=index).

## Documentation

### Contents

1. [Components](https://github.com/trustmaster/goflow/wiki/Components)
    1. [Ports, Events and Handlers](https://github.com/trustmaster/goflow/wiki/Components#ports-events-and-handlers)
    2. [Processes and their lifetime](https://github.com/trustmaster/goflow/wiki/Components#processes-and-their-lifetime)
    3. [State](https://github.com/trustmaster/goflow/wiki/Components#state)
    4. [Concurrency](https://github.com/trustmaster/goflow/wiki/Components#concurrency)
    5. [Internal state and Thread-safety](https://github.com/trustmaster/goflow/wiki/Components#internal-state-and-thread-safety)
2. [Graphs](https://github.com/trustmaster/goflow/wiki/Graphs)
    1. [Structure definition](https://github.com/trustmaster/goflow/wiki/Graphs#structure-definition)
    2. [Behavior](https://github.com/trustmaster/goflow/wiki/Graphs#behavior)

### Package docs

Documentation for the flow package can be accessed using standard godoc tool, e.g.

```
godoc github.com/trustmaster/goflow
```

## More examples

* [GoChat](https://github.com/trustmaster/gochat), a simple chat in Go using this library

## Links

Here are related projects and resources:

* [J. Paul Morrison's Flow-Based Programming](http://www.jpaulmorrison.com/fbp/), the origin of FBP, [JavaFBP, C#FBP](http://sourceforge.net/projects/flow-based-pgmg/) and [DrawFBP](http://www.jpaulmorrison.com/fbp/#DrawFBP) diagramming tool.
* [Knol about FBP](http://knol.google.com/k/flow-based-programming)
* [NoFlo](http://noflojs.org/), FBP for JavaScript and Node.js
* [Pypes](http://www.pypes.org/), flow-based Python ETL
* [Go](http://golang.org/), the Go programming language

## TODO

* Integration with NoFlo-UI
* Distributed networks via TCP/IP and UDP
* Better run-time restructuring and evolution
* Reflection and monitoring of networks
