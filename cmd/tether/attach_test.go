// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"sync"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"

	"github.com/vmware/vic/metadata"
	"github.com/vmware/vic/portlayer/attach"
)

func addKey(config *metadata.ExecutorConfig) (*metadata.ExecutorConfig, error) {

	// generate a host key for the tether
	privateKey, err := rsa.GenerateKey(rand.Reader, 512)
	if err != nil {
		return nil, err
	}

	privateKeyDer := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privateKeyDer,
	}

	config.Key = pem.EncodeToMemory(&privateKeyBlock)
	return config, nil
}

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachConfig sets up the config for attach testing - the grep will echo anything
// sent and adds colour which is useful for tty testing
//
type TestAttachConfig struct{}

func (c *TestAttachConfig) StoreConfig(*metadata.ExecutorConfig) (string, error) {
	return "", errors.New("not implemented")
}
func (c *TestAttachConfig) LoadConfig() (*metadata.ExecutorConfig, error) {
	config := metadata.ExecutorConfig{}

	config.ID = "attach"
	config.Name = "tether_test_executor"
	config.Sessions = map[string]metadata.SessionConfig{
		"attach": metadata.SessionConfig{
			Common: metadata.Common{
				ID:   "attach",
				Name: "tether_test_session",
			},
			Tty:    false,
			Attach: true,
			Cmd: metadata.Cmd{
				Path: "/usr/bin/tee",
				// grep, matching everything, reading from stdin
				Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
				Env:  []string{},
				Dir:  "/",
			},
		},
	}

	return addKey(&config)
}

func TestAttach(t *testing.T) {
	testSetup(t)
	defer testTeardown(t)

	testServer, _ := server.(*testAttachServer)

	// if there's no session command with guaranteed exit then tether needs to run in the background
	cfg := &TestAttachConfig{}
	testConfig, err := cfg.LoadConfig()
	if err != nil {
		t.Error(err)
		return
	}

	// run the tether to service the attach
	go func() {
		erR := run(cfg)
		if erR != nil {
			t.Error(erR)
		}
	}()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Error("attach server was not enabled")
		return
	}

	// create client on the mock pipe
	conn, err := mockSerialConnection(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	containerConfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client from the mocked connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", containerConfig)
	if !assert.NoError(t, err) {
		return
	}
	defer sshConn.Close()

	attachClient := ssh.NewClient(sshConn, chans, reqs)

	sshSession, err := attach.SSHAttach(attachClient, testConfig.ID)
	if err != nil {
		t.Error(err)
		return
	}

	stdout := sshSession.Stdout()

	// FIXME: the pipe pair are line buffered - how do I disable that so we
	// don't have odd hangs to diagnose when the trailing \n is missed

	testBytes := []byte("\x1b[32mhello world\x1b[39m!\n")
	// read from session into buffer
	buf := &bytes.Buffer{}
	done := make(chan bool)
	go func() { io.CopyN(buf, stdout, int64(len(testBytes))); done <- true }()

	// write something to echo
	log.Debug("sending test data")
	sshSession.Stdin().Write(testBytes)
	log.Debug("sent test data")

	// wait for the close to propogate
	<-done
	sshSession.Stdin().Close()

	if !assert.Equal(t, buf.Bytes(), testBytes) {
		return
	}
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachTTYConfig sets up the config for attach testing
//
type TestAttachTTYConfig struct{}

func (c *TestAttachTTYConfig) StoreConfig(*metadata.ExecutorConfig) (string, error) {
	return "", errors.New("not implemented")
}
func (c *TestAttachTTYConfig) LoadConfig() (*metadata.ExecutorConfig, error) {
	config := metadata.ExecutorConfig{}

	config.ID = "attach"
	config.Name = "tether_test_executor"
	config.Sessions = map[string]metadata.SessionConfig{
		"attach": metadata.SessionConfig{
			Common: metadata.Common{
				ID:   "attach",
				Name: "tether_test_session",
			},
			Tty:    true,
			Attach: true,
			Cmd: metadata.Cmd{
				Path: "/usr/bin/tee",
				// grep, matching everything, reading from stdin
				Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
				Env:  []string{},
				Dir:  "/",
			},
		},
	}

	return addKey(&config)
}

func TestAttachTTY(t *testing.T) {
	t.Skip("TTY test skipped - not sure how to test this correctly")

	testSetup(t)
	defer testTeardown(t)

	testServer, _ := server.(*testAttachServer)

	// if there's no session command with guaranteed exit then tether needs to run in the background
	cfg := &TestAttachTTYConfig{}
	testConfig, err := cfg.LoadConfig()
	if err != nil {
		t.Error(err)
		return
	}

	go func() {
		erR := run(cfg)
		if erR != nil {
			t.Error(erR)
		}
	}()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Error("attach server was not enabled")
		return
	}

	// create client on the mock pipe
	conn, err := mockSerialConnection(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	cconfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client
	sConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", cconfig)
	if err != nil {
		t.Error(err)
		return
	}
	defer sConn.Close()
	client := ssh.NewClient(sConn, chans, reqs)

	session, err := attach.SSHAttach(client, testConfig.ID)
	if err != nil {
		t.Error(err)
		return
	}

	stdout := session.Stdout()

	// FIXME: this is line buffered - how do I disable that so we don't have odd hangs to diagnose
	// when the trailing \n is missed
	testBytes := []byte("\x1b[32mhello world\x1b[39m!\n")
	// after tty translation the above string should result in the following
	refBytes := []byte("\x5e[[32mhello world\x5e[[39m!\n")

	// read from session into buffer
	buf := &bytes.Buffer{}

	var wg sync.WaitGroup
	go func() { wg.Add(1); io.CopyN(buf, stdout, int64(len(refBytes))); wg.Done() }()

	// write something to echo
	log.Debug("sending test data")
	session.Stdin().Write(testBytes)
	log.Debug("sent test data")

	// wait for the close to propogate
	wg.Wait()
	session.Stdin().Close()

	if !assert.Equal(t, refBytes, buf.Bytes()) {
		return
	}
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachTwoConfig sets up the config for attach testing - tests launching and
// attaching to two different processes simultaneously
//
type TestAttachTwoConfig struct{}

func (c *TestAttachTwoConfig) StoreConfig(*metadata.ExecutorConfig) (string, error) {
	return "", errors.New("not implemented")
}
func (c *TestAttachTwoConfig) LoadConfig() (*metadata.ExecutorConfig, error) {
	config := metadata.ExecutorConfig{}

	config.ID = "attachtwo"
	config.Name = "tether_test_executor"
	config.Sessions = map[string]metadata.SessionConfig{
		"tee1": metadata.SessionConfig{
			Common: metadata.Common{
				ID:   "tee1",
				Name: "tether_test_session1",
			},
			Tty:    false,
			Attach: true,
			Cmd: metadata.Cmd{
				Path: "/usr/bin/tee",
				// grep, matching everything, reading from stdin
				Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
				Env:  []string{},
				Dir:  "/",
			},
		},
		"tee2": metadata.SessionConfig{
			Common: metadata.Common{
				ID:   "tee2",
				Name: "tether_test_session2",
			},
			Tty:    false,
			Attach: true,
			Cmd: metadata.Cmd{
				Path: "/usr/bin/tee",
				// grep, matching everything, reading from stdin
				Args: []string{"/usr/bin/tee", pathPrefix + "/tee2.out"},
				Env:  []string{},
				Dir:  "/",
			},
		},
	}

	return addKey(&config)
}

func TestAttachTwo(t *testing.T) {

	testSetup(t)
	defer testTeardown(t)

	testServer, _ := server.(*testAttachServer)

	// if there's no session command with guaranteed exit then tether needs to run in the background
	cfg := &TestAttachTwoConfig{}
	_, err := cfg.LoadConfig()
	if err != nil {
		t.Error(err)
		return
	}

	go func() {
		erR := run(cfg)
		if erR != nil {
			t.Error(erR)
		}
	}()

	// wait for updates to occur
	<-mocked.started
	<-testServer.updated

	if !testServer.enabled {
		t.Error("attach server was not enabled")
		return
	}

	// create client on the mock pipe
	conn, err := mockSerialConnection(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	cconfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client
	sConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", cconfig)
	if err != nil {
		t.Error(err)
		return
	}
	defer sConn.Close()
	client := ssh.NewClient(sConn, chans, reqs)

	ids, err := attach.SSHls(client)
	if err != nil {
		t.Error(err)
		return
	}

	// there's no ordering guarantee in the returned ids
	if len(ids) != 2 {
		t.Errorf("ID list - expected 2, got %d", len(ids))
		return
	}

	reference, _ := cfg.LoadConfig()
	for _, id := range ids {
		if _, ok := reference.Sessions[id]; !ok {
			t.Errorf("Expected sessions to have an entry for %s", id)
			return
		}
		delete(reference.Sessions, id)
	}

	sessionA, err := attach.SSHAttach(client, "tee1")
	if err != nil {
		t.Error(err)
		return
	}

	sessionB, err := attach.SSHAttach(client, "tee2")
	if err != nil {
		t.Error(err)
		return
	}

	stdoutA := sessionA.Stdout()
	stdoutB := sessionB.Stdout()

	// FIXME: this is line buffered - how do I disable that so we don't have odd hangs to diagnose
	// when the trailing \n is missed
	testBytesA := []byte("hello world!\n")
	testBytesB := []byte("goodbye world!\n")
	// read from session into buffer
	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}

	var wg sync.WaitGroup
	// wg.Add cannot go inside the go routines as the Add may not have happened by the time we
	// call Wait
	wg.Add(2)
	go func() { io.CopyN(bufA, stdoutA, int64(len(testBytesA))); wg.Done() }()
	go func() { io.CopyN(bufB, stdoutB, int64(len(testBytesB))); wg.Done() }()

	// write something to echo
	log.Debug("sending test data")
	sessionA.Stdin().Write(testBytesA)
	sessionB.Stdin().Write(testBytesB)
	log.Debug("sent test data")

	// wait for the close to propogate
	wg.Wait()
	sessionA.Stdin().Close()
	sessionB.Stdin().Close()

	<-mocked.cleaned

	if !assert.Equal(t, bufA.Bytes(), testBytesA) {
		return
	}

	if !assert.Equal(t, bufB.Bytes(), testBytesB) {
		return
	}
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachInvalid sets up the config for attach testing - launches a process but
// tries to attach to an invalid session id
//
type TestAttachInvalidConfig struct{}

func (c *TestAttachInvalidConfig) StoreConfig(*metadata.ExecutorConfig) (string, error) {
	return "", errors.New("not implemented")
}
func (c *TestAttachInvalidConfig) LoadConfig() (*metadata.ExecutorConfig, error) {
	config := metadata.ExecutorConfig{}

	config.ID = "attachinvalid"
	config.Name = "tether_test_executor"
	config.Sessions = map[string]metadata.SessionConfig{
		"valid": metadata.SessionConfig{
			Common: metadata.Common{
				ID:   "valid",
				Name: "tether_test_session",
			},
			Tty:    false,
			Attach: true,
			Cmd: metadata.Cmd{
				Path: "/usr/bin/tee",
				// grep, matching everything, reading from stdin
				Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
				Env:  []string{},
				Dir:  "/",
			},
		},
	}

	return addKey(&config)
}

func TestAttachInvalid(t *testing.T) {
	testSetup(t)
	defer testTeardown(t)

	testServer, _ := server.(*testAttachServer)

	// if there's no session command with guaranteed exit then tether needs to run in the background
	cfg := &TestAttachInvalidConfig{}
	_, err := cfg.LoadConfig()
	if err != nil {
		t.Error(err)
		return
	}

	go func() {
		erR := run(cfg)
		if erR != nil {
			t.Error(erR)
		}
	}()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Error("attach server was not enabled")
		return
	}

	// create client on the mock pipe
	conn, err := mockSerialConnection(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	cconfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client
	sConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", cconfig)
	if err != nil {
		t.Error(err)
		return
	}
	defer sConn.Close()
	client := ssh.NewClient(sConn, chans, reqs)

	_, err = attach.SSHAttach(client, "invalidID")
	if err != nil {
		t.Log(err)
		return
	}

	t.Error("Expected to fail on attempt to attach to invalid session")
}

//
/////////////////////////////////////////////////////////////////////////////////////
