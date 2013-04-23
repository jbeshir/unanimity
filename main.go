package main

import (
	"bufio"
	"crypto/tls"
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
	"github.com/jbeshir/unanimity/client"
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/core"
)

func main() {
	// Define and parse flags.
	id := flag.Uint("id", 0, "Set the node ID of this unanimity instance.")
	flag.Parse()

	// Validate flags.
	if *id == 0 || *id > 0xFFFF {
		log.Fatal("Invalid node ID specified.")
	}

	// Load configuration from config file.
	loadConfig()
	config.SetId(uint16(*id))

	// Load our TLS certificate.
	idStr := strconv.FormatUint(uint64(config.Id()), 10)
	cert, err := tls.LoadX509KeyPair(idStr+".crt", idStr+".key")
	if err != nil {
		log.Fatalf("error loading our TLS certificate: %s", err)
	}
	config.SetCertificate(&cert)

	fmt.Printf("Loaded configuration, our ID is %d.\n", config.Id())
	if config.IsCore() {
		core.Startup()
	} else {
		client.Startup()
	}
	select {}
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
