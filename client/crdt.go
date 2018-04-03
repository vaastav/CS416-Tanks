package main

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"../crdtlib"
	"strconv"
	"strings"
)

// Sets up the key-value store by reading any existing key-value pairs stored on
// disk, and returns a map populated by them.
func KVStoreSetup() (map[uint64]crdtlib.ValueType, error) {

	M := make(map[uint64]crdtlib.ValueType)

	files, err := filepath.Glob(path.Join(".", KVDir, "*.kv"))
	if err != nil {
		return M, err
	}

	for _, file := range files {
		_, fname := path.Split(file)
		fname = strings.Split(fname, ".")[0]
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return M, err
		}
		dataStr := strings.Split(string(data), "\n")
		v0, _ := strconv.Atoi(dataStr[0])
		v1, _ := strconv.Atoi(dataStr[1])
		k, _ := strconv.Atoi(fname)
		val := crdtlib.ValueType{v0, v1}
		M[uint64(k)] = val
	}

	return M, nil
}

func KVGet(key uint64) (crdtlib.ValueType, error) {

	reply, err := Server.KVGet(key, NetworkSettings.UniqueUserID, KVLogger)
	if err != nil {
		return reply.Value, err
	}
	if reply.HasAlready {
		KVMap.Lock()
		defer KVMap.Unlock()
		return KVMap.M[key], nil
	} else {
		return reply.Value, nil
	}

}

func KVPut(key uint64, value crdtlib.ValueType) error {

	err := Server.KVPut(key, value, KVLogger)

	return err

}
