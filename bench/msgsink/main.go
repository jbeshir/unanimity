/*
	Benchmark test program for unanimity.

	Connects, then listens for benchmark messages.

	Intended for testing latency, message loss, and throughput.

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
	"strings"
	"time"
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

var print *uint

func main() {
	// Define and parse flags.
	id := flag.Uint("id", 0, "Set the client node ID to connect to.")
	print = flag.Uint("print", 10000, "Sets number of messages to print " +
		"after.")

	flag.Parse()

	// Validate flags.
	if *id == 0 || *id > 0xFFFF {
		log.Fatal("Invalid node ID specified.")
	}

	// Load configuration from config file.
	loadConfig()

	fmt.Printf("Loaded configuration, ID to connect to is %d.\n", *id)

	conn, err := connect.TestDial(uint16(*id))
	if err != nil {
		log.Fatal(err)
	}

	var msg cliproto_up.Authenticate
	msg.Username = new(string)
	msg.Password = new(string)
	msg.SessionId = new(uint64)
	*msg.Username = "msgsink"
	*msg.Password = "msgsink"
	conn.SendProto(2, &msg)

	for {
		respMsg, ok := <-conn.Received
		if !ok {
			break
			log.Fatal("connection error")
		}

		switch *respMsg.MsgType {
		case 2:
			conn.Close()
			log.Fatal("auth failed")
		case 3:
			log.Print("authenticated")
		case 10:
			handleMsg(respMsg.Content)
		}
	}
}

var started bool
var count uint64
var start time.Time
var delay time.Duration

func handleMsg(content []byte) {
	var msg cliproto_down.Received
	if err := proto.Unmarshal(content, &msg); err != nil {
		log.Fatal(err)
	}

	if *msg.Tag != "benchmark" {
		return
	}

	// Provide a count of messages.
	count++

	// Provide latency measuring.
	// Note that for this, msgsource and msgsink need
	// *very* closely matched system clocks, with the difference
	// small compared to expected latency.
	parts := strings.Split(*msg.Content, " ")
	timeUnixNanos, _ := strconv.ParseInt(parts[0], 10, 64)
	delay += time.Now().Sub(time.Unix(0, timeUnixNanos))

	if count % uint64(*print) == 0 {

		// We discard the first period's results because they are
		// contaminated by connection setup delays, which while
		// somewhat interesting are not what we are measuring.
		if !started {
			count = 0
			start = time.Now()
			delay = 0

			started = true
			return
		}

		msgIndex, _ := strconv.ParseUint(parts[1], 10, 64)
		loss := 1 - (float64(count+uint64(*print)) / float64(msgIndex))
		runningSecs := float64(time.Now().Sub(start) / time.Second)

		log.Print("count: ", count)
		if runningSecs > 0 {
			log.Print("msgs/sec: ", float64(count) / runningSecs)
		}
		log.Print("average latency: ", delay / time.Duration(count))
		log.Print("approx msg loss: ", loss * 100, "%")
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
