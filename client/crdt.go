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
func KVStoreSetup() (map[int]crdtlib.ValueType, error) {

	M := make(map[int]crdtlib.ValueType)

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
		M[k] = val
	}

	return M, nil
}

func KVGet(key int) (crdtlib.ValueType, error) {

	reply, err := Server.KVGet(key, NetworkSettings.UniqueUserID)
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

func KVPut(key int, value crdtlib.ValueType) error {

	err := Server.KVPut(key, value)

	return err

}
