/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
)

// A Dialer is a means to establish a connection.
type Dialer interface {
	// Dial can connect to an address via a proxy.
	Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
}

type directDial struct{}

func (d directDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	return ssh.Dial(network, addr, config)
}

type proxyDial struct {
	proxyHost string
}

func (d proxyDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	// build a proxy connection first
	pconn, err := dialProxy(d.proxyHost, addr)
	if err != nil {
		return nil, err
	}

	// do the same as ssh.Dial but pass in proxy connection
	c, chans, reqs, err := ssh.NewClientConn(pconn, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// FromEnvironment returns a Dial function. If the https_proxy or http_proxy
// environment variable are set, it returns a function that will dial through
// said proxy server. If neither variable is set, it will connect to the SSH
// server directly.
func FromEnvironment() Dialer {
	// try to get proxy address from environment
	var proxyAddr string
	proxyAddr = os.Getenv("https_proxy")
	if proxyAddr == "" {
		proxyAddr = os.Getenv("http_proxy")
	}

	// if no proxy settings are in environment return regular ssh dialer,
	// otherwise return a proxy dialer
	if proxyAddr == "" {
		return directDial{}
	}
	return proxyDial{proxyHost: proxyAddr}
}

func dialProxy(proxyAddr string, addr string) (net.Conn, error) {
	ctx := context.Background()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}
	connectReq.Write(conn)

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}
	if resp.StatusCode != 200 {
		f := strings.SplitN(resp.Status, " ", 2)
		conn.Close()
		return nil, trace.BadParameter("Unable to proxy connection, unexpected StatusCode %v: %v", resp.StatusCode, f[1])
	}

	return conn, nil
}

// ConnectHandler is used in tests to debug HTTP CONNECT connections.
func ConnectHandler(w http.ResponseWriter, r *http.Request) {
	// validate http connect parameters
	if r.Method != http.MethodConnect {
		http.Error(w, fmt.Sprintf("%v not supported", r.Method), http.StatusInternalServerError)
		return
	}
	if r.Host == "" {
		http.Error(w, fmt.Sprintf("host not set"), http.StatusInternalServerError)
		return
	}

	// hijack request so we can get underlying connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "unable to hijack connection", http.StatusInternalServerError)
		return
	}
	sconn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// dial to host we want to proxy connection to
	dconn, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// write 200 OK to the source, but don't close the connection
	resp := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
	}
	resp.Write(sconn)

	// copy from src to dst and dst to src
	done := make(chan bool)
	go func() {
		io.Copy(sconn, dconn)
		done <- true
	}()
	go func() {
		io.Copy(dconn, sconn)
		done <- true
	}()

	// wait until done
	<-done
	<-done

	// close the connections
	sconn.Close()
	dconn.Close()
}
