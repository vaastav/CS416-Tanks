/*

This package specifies the applications's interface to a conflict free replicated data store (CRDT) to be used in project 2 of UBC CS 416 2017W2.

*/

package crdtlib

/*

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"path"
	"runtime"
	"strconv"
	"sync"
)

*/

// ----------------------------------------------------------------------------

// Define types.

// A int represents the type of keys in the key-value store.
// type int int

// A Value represents the type of values in the key-value store.
type ValueType struct {
	NumKills  int
	NumDeaths int
}

// A GetArg represents an argument type passed when a client the server an RPC
// to get the value of a key.
type GetArg struct {
	ClientId uint64
	Key      int
}

// A GetReply represents the reply sent from the server to the client on a Get
// call.
type GetReply struct {
	Ok          bool
	HasAlready  bool
	Unavailable bool
	Value       ValueType
}

// A PutArg represents an argument type passed when a client sends the server an
// RPC to write a key-value pair.
type PutArg struct {
	Key   int
	Value ValueType
}

// A PutReply represents the reply sent from the server to the client on a Put
// call.
type PutReply struct {
	Ok bool
}
