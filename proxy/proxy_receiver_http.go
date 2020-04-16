package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"io/ioutil"
	"sync"
	"strconv"
	"time"

	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/contact"
)

var httpProxyName = "HTTP"
var defaultPort = 61889

//HttpReceiver forwards data received from HTTP requests to the upstream server via HTTP. Implements the P2pReceiver interface.
type HttpReceiver struct {
	upstreamServer string
	port int
	receiverName string
	upstreamComs contact.Contact
	httpServer *http.Server
	waitgroup *sync.WaitGroup
	receiverContext context.Context
	receiverCancelFunc context.CancelFunc
}

func init() {
	P2pReceiverChannels[httpProxyName] = &HttpReceiver{}
}

/*
InitializeReceiver(server string, upstreamComs contact.Contact) error
RunReceiver()
UpdateUpstreamServer(newServer string)
UpdateUpstreamComs(newComs contact.Contact)
Terminate()
*/

func (h *HttpReceiver) InitializeReceiver(server string, upstreamComs contact.Contact, waitgroup *sync.WaitGroup) error {
	// Make sure the agent uses HTTP with the C2.
	switch upstreamComs.(type) {
	case contact.API:
		h.upstreamServer = server
		h.port = defaultPort
		h.receiverName = httpProxyName
		h.upstreamComs = upstreamComs
		h.httpServer = &http.Server{
			Addr: ":" + string(h.port),
			Handler: nil,
		}
		h.waitgroup = waitgroup
		h.receiverContext, h.receiverCancelFunc = context.WithTimeout(context.Background(), 5*time.Second)
		return nil
	default:
		return errors.New("Cannot initialize HTTP proxy receiver if agent is not using HTTP communication with the C2.")
	}
}

func (h *HttpReceiver) RunReceiver() {
	output.VerbosePrint(fmt.Sprintf("[*] Starting HTTP proxy receiver on local port %d", h.port))
	output.VerbosePrint(fmt.Sprintf("[*] HTTP proxy receiver has upstream contact at %s", h.upstreamServer))
	h.startHttpProxy()
}

func (h *HttpReceiver) Terminate() {
	defer func() {
		h.waitgroup.Done()
		h.receiverCancelFunc()
	}()

	if err := h.httpServer.Shutdown(h.receiverContext); err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Error when shutting down HTTP receiver server: %s", err.Error()))
	} else {
		output.VerbosePrint("[-] Shut down HTTP receiver server.")
	}
}

func (h *HttpReceiver) UpdateUpstreamServer(newServer string) {
	h.upstreamServer = newServer
}

func (h *HttpReceiver) UpdateUpstreamComs(newComs contact.Contact) {
	switch newComs.(type) {
	case contact.API:
		h.upstreamComs = newComs
	default:
		output.VerbosePrint("[-] Cannot switch to non-HTTP comms.")
	}
}

// Helper method for StartReceiver. Starts HTTP proxy to forward messages from peers to the C2 server.
func (h *HttpReceiver) startHttpProxy() {
	defer h.waitgroup.Done()
	listenPort := ":" + strconv.Itoa(h.port)
	proxyHandler := func(writer http.ResponseWriter, reader *http.Request) {
		// Get data from the message that client peer sent.
		httpClient := http.Client{}
		body, err := ioutil.ReadAll(reader.Body)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		reader.Body = ioutil.NopCloser(bytes.NewReader(body))

		// Determine where to forward the request.
		url := h.upstreamServer + reader.RequestURI

		// Forward the request to the C2 server, and send back the response.
		proxyReq, err := http.NewRequest(reader.Method, url, bytes.NewReader(body))
		if err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] Error creating new HTTP request: %s", err.Error()))
			return
		}
		proxyReq.Header = make(http.Header)
		for header, val := range reader.Header {
			proxyReq.Header[header] = val
		}
		resp, err := httpClient.Do(proxyReq)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		bites, _ := ioutil.ReadAll(resp.Body)
		writer.Write(bites)
	}
	http.HandleFunc("/", proxyHandler)
	output.VerbosePrint(listenPort + "ddd")
	output.VerbosePrint(fmt.Sprintf("[*] %s", http.ListenAndServe(listenPort, nil)))
}