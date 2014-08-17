package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type State struct {
	On       bool      `json:"on"`
	Reported time.Time `json:"reported"`
}

func main() {
	/*
		f, err := os.OpenFile("example-switch.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalln("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	*/
	log.SetOutput(os.Stderr)

	exitCh := make(chan os.Signal, 1)
	stateCh := make(chan bool)

	// Stdin reader
	go func() {
		log.Println("Starting stdin scanning...")
		scanner := bufio.NewScanner(os.Stdin)
		var s State
		for scanner.Scan() {
			b := scanner.Bytes()
			log.Println("Received bytes:", b, ", String:", string(b))
			err := json.Unmarshal(b, &s)
			if err != nil {
				log.Println("ERROR", err.Error())
				continue
			}
			log.Println("Received from stdin:", s)
			stateCh <- s.On
		}
		if err := scanner.Err(); err != nil {
			log.Println("Error scanning:", err.Error())
			exitCh <- syscall.SIGTERM
		}
	}()

	// State reporter
	go func() {
		state := new(State)
		for {
			select {
			case v := <-stateCh:
				state.On = v
				state.Reported = time.Now()
				data, _ := json.Marshal(state)
				fmt.Println(string(data))
			case <-time.Tick(time.Duration(10) * time.Second):
				state.Reported = time.Now()
				data, _ := json.Marshal(state)
				fmt.Println(string(data))
			}
		}
	}()

	// Ctrl+C handling
	signal.Notify(exitCh, os.Interrupt, syscall.SIGTERM)
	for _ = range exitCh {
		break
	}

	os.Exit(0)
}
