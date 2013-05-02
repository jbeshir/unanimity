/*
	Benchmark test program for unanimity.

	Connects, then continuously sends messages to the first session of the
	"msgsink" remote user.

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

var rate *uint

func main() {
	// Define and parse flags.
	id := flag.Uint("id", 0, "Set the client node ID to connect to.")
	rate = flag.Uint("rate", 0, "Sets the maximum messages per second.")
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
	*msg.Username = "msgsource"
	*msg.Password = "msgsource"
	conn.SendProto(2, &msg)

	for {
		respMsg, ok := <-conn.Received
		if !ok {
			log.Fatal("connection error")
		}

		switch *respMsg.MsgType {
		case 2:
			conn.Close()
			log.Fatal("auth failed")
		case 3:
			log.Print("authenticated, follow msgsink...")

			var followMsg cliproto_up.FollowUsername
			followMsg.Username = new(string)
			*followMsg.Username = "msgsink"
			conn.SendProto(3, &followMsg)
		case 4:
			log.Fatal("follow failed")
		case 6:
			handleData(respMsg.Content)
		case 7:
			log.Print("following msgsink, sending msgs...")
			go sendMessages(conn)
		}
	}
}

var sinkUserId uint64
var sinkUserData = make(map[string]string)

func handleData(content []byte) {
	var msg cliproto_down.UserData
	if err := proto.Unmarshal(content, &msg); err != nil {
		log.Fatal(err)
	}

	sinkUserId = *msg.UserId
	sinkUserData[*msg.Key] = *msg.Value
}

func sendMessages(conn *connect.BaseConn) {

	var sessionId uint64
	for key := range sinkUserData {
		if strings.HasPrefix(key, "attach ") {
			idStr := key[7:]
			sessionId, _ = strconv.ParseUint(idStr, 10, 64)
		}
	}

	var msg cliproto_up.Send
	msg.Recipient = new(uint64)
	msg.Tag = new(string)
	msg.Content = new(string)
	*msg.Recipient = sessionId
	*msg.Tag = "benchmark"

	i := uint64(0)
	prevSec := time.Now().Unix()
	count := uint(0)
	for {
		sec := time.Now().Unix()
		if sec != prevSec {
			prevSec = sec
			count = 0
		}

		// If we've hit our cap, busyloop until we can send again.
		if *rate != 0 && count >= *rate {
			continue
		}

		i++
		count++

		timeStr := strconv.FormatInt(time.Now().UnixNano(), 10)
		numberStr := strconv.FormatUint(i, 10)
		*msg.Content = timeStr + " " + numberStr

		conn.SendProto(6, &msg)
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
