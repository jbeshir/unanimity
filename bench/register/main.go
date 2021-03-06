/*
	Benchmark test program for unanimity.

	Creates a specified number of registered accounts, sequentially.
	Intended for testing how RAM usage scales with user entities.
*/
package main

import (
	"bufio"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
)

import (
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/client/listener/cliproto_down"
	"github.com/jbeshir/unanimity/client/listener/cliproto_up"
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/connect"
)

func main() {
	// Define and parse flags.
	conns := flag.Uint("conns", 20, "Sets number of connections to use.")
	count := flag.Uint("count", 1000, "Number of users to create.")
	flag.Parse()

	// Validate flags.
	if *conns == 0 {
		log.Fatal("Number of connections must be at least 1.")
	}
	if *count % *conns != 0 {
		log.Fatal("Please use a number of users which divides " +
			"into the number of connections.")
	}

	// Load configuration from config file.
	loadConfig()

	log.Print("Loaded configuration, starting...")

	// Launch the specified number of goroutines registering users.
	perConnCount := *count / *conns
	doneChan := make(chan bool, *conns)

	clientNodes := config.ClientNodes()
	for i := 0; i < int(*conns); i++ {
		node := clientNodes[i % len(clientNodes)]
		prefix := "Test-" + strconv.FormatUint(uint64(i), 10) + "-"
		go register(node, prefix, perConnCount, doneChan)
	}

	// Wait until they're all done.
	for i := uint(0); i < *conns; i++ {
		<-doneChan
	}
}

func register(node uint16, prefix string, limit uint, doneChan chan bool) {

	for i := uint(0); i < limit; i++ {
		conn, err := connect.TestDial(node)
		if err != nil {
			log.Fatal(err)
		}

		var msg cliproto_up.Authenticate
		msg.Username = new(string)
		msg.Password = new(string)
		msg.SessionId = new(uint64)
		*msg.Username = prefix + strconv.FormatUint(uint64(i), 10)
		*msg.Password = prefix + strconv.FormatUint(uint64(i), 10)
		conn.SendProto(2, &msg)

	connloop:
		for {
			respMsg, ok := <-conn.Received
			if !ok {
				log.Fatal("connection error to ", node)
				break connloop
			}

			switch *respMsg.MsgType {
			case 2:
				// Terminates.
				handleAuthFail(respMsg.Content)
			case 3:
				conn.Close()
				break connloop
			}
		}
	}

	doneChan <- true
}

func handleAuthFail(content []byte) {
	var msg cliproto_down.AuthenticationFailed
	if err := proto.Unmarshal(content, &msg); err != nil {
		log.Fatal(err)
	}

	log.Fatal("auth failed: ", *msg.Reason)
}

func loadConfig() {
	// Reads a list of node IDs and IPs from unanimity.conf
	// This configuration mechanism is simple but may still want replacing
	// with a configuration program of some kind.

	// Open config file.
	configFile, err := os.Open("unanimity.conf")
	if err != nil {
		log.Fatal(err)
	}

	// Read each line of the file, discarding if it begins with #,
	// and otherwise treating as an ID followed by a space then IP
	// for another node.
	configReader := bufio.NewReader(configFile)
	line, err := configReader.ReadString('\n')
	lineNum := 0
	for line != "" {
		if err != nil {
			if err == io.EOF {
				log.Fatal("config missing final newline")
			}

			log.Fatal(err)
		}

		lineNum++
		if line[0] != '#' {

			// Read node ID and IP from the line.
			id := new(uint16)
			ipStr := new(string)
			_, err = fmt.Sscanf(line, "%d %s", id, ipStr)
			if err != nil {
				log.Fatalf("config line %d: %s", lineNum, err)
			}

			ip := net.ParseIP(*ipStr)
			if ip == nil {
				log.Fatalf("config line %d: invalid IP",
					lineNum)
			}

			// Load node certificate.
			idStr := strconv.FormatUint(uint64(*id), 10)
			cert := loadCertFile(idStr + ".crt")

			// Add node to configuration.
			config.AddNode(*id, ip, cert)
		}

		line, err = configReader.ReadString('\n')
	}
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
}

func loadCertFile(filename string) *x509.CertPool {
	cert := x509.NewCertPool()
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("error loading node certificate file: %s", err)
	}

	ok := cert.AppendCertsFromPEM(file)
	if !ok {
		log.Fatal("unable to parse node certificate file: " + filename)
	}

	return cert
}
