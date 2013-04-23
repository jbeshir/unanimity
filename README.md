unanimity
=========

Fault-tolerant message relay system using Paxos. Presently only a proof of concept due to its in-dev, experimental, prototype status, i.e. lack of unit testing, comprehensive testing in general, or hardening against malicious input.

This implementation may be used as guidance for implementing a more hardened system with good test coverage at a later point, or refactored into one.

Implemented in Go, targeting Go 1.1 at present. Depends on the following:

* "code.google.com/p/goprotobuf/proto" - Go support for Google protocol buffers.
* "code.google.com/p/go.crypto/scrypt" - Implementation of the scrypt password hashing algorithm. Used for password hashes.
