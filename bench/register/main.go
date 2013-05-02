/*
	Benchmark test program for unanimity.

	Creates a specified number of registered accounts, sequentially.
	Waits for each to be registered before making the next.
	Intended for testing how RAM usage scales with user entities.

	Connects only to a single node.
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
	"github.com/jbeshir/unanimity/client/listener/cliproto_up"
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/connect"
)

func main() {
	// Define and parse flags.
	id := flag.Uint("id", 0, "Set the client node ID to connect to.")
	count := flag.Uint("count", 1000, "Number of users to create.")
	flag.Parse()

	// Validate flags.
	if *id == 0 || *id > 0xFFFF {
		log.Fatal("Invalid node ID specified.")
	}

	// Load configuration from config file.
	loadConfig()

	fmt.Printf("Loaded configuration, ID to connect to is %d.\n", *id)
	for i := uint(0); i < *count; i++ {
		conn, err := connect.TestDial(uint16(*id))
		if err != nil {
			log.Fatal(err)
		}

		var msg cliproto_up.Authenticate
		msg.Username = new(string)
		msg.Password = new(string)
		msg.SessionId = new(uint64)
		*msg.Username = "Test" + strconv.FormatUint(uint64(i), 10)
		*msg.Password = "Test" + strconv.FormatUint(uint64(i), 10)
		conn.SendProto(2, &msg)

	connloop:
		for {
			respMsg, ok := <-conn.Received
			if !ok {
				break connloop
				log.Fatal("connection error")
			}

			switch *respMsg.MsgType {
			case 3:
				conn.Close()
				break connloop
			}
		}
	}
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
