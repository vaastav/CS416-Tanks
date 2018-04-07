package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func testValUniqueness(s string) (b bool) {
	allStrings := strings.Split(s, " ")
	values := make(map[string]bool)
	for i, str := range allStrings {
		if i == 0 {
			k := strings.Split(str,":")
			key := strings.	Split(k[0], "[")[1]
			if _, ok := values[key]; ok {
				return false
			}
			values[key] = true
		} else {
			k := strings.Split(str, ":")
			if _, ok := values[k[0]]; ok {
				return false
			}
			values[k[0]] = true
		}
	}
	return true
}

func parseValues(s string) (m map[string]bool) {
	allStrings := strings.Split(s, " ")
	values := make(map[string]bool)
	for i, str := range allStrings {
		if i == 0 {
			k := strings.Split(str,":")
			key := strings.	Split(k[0], "[")[1]
			values[key] = true
		} else {
			k := strings.Split(str, ":")
			values[k[0]] = true
		}
	}
	return values
}

func main() {
	dirname := "." + string(filepath.Separator)
	
	d, err := os.Open(dirname)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	
	files, err := d.Readdir(-1)
	if err != nil {
		log.Fatal(err)
	}
	
	log.Println("Reading "+ dirname)

	var s []string
	for _, file := range files {
		if file.Mode().IsRegular() {
			if filepath.Ext(file.Name()) == ".dtrace" {
				s = append(s, file.Name())
			}
		}
	}

	log.Println("TESTING INVARIANT 1 : Server's list of display names have unique values")

	isUnique := true
	for _, fName := range s {
		f, err := os.Open(dirname + fName)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		foundVariable := false
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			s2 := scanner.Text()
			if foundVariable {
				foundVariable = false
				isUnique = isUnique && testValUniqueness(s2)
			} 
			if strings.Contains(s2, "displayNames") {
				if !strings.HasPrefix(s2, "variable") {
					foundVariable = true
				}
			}
		}
	}

	if isUnique {
		log.Println("Invariant Holds")
	} else {
		log.Println("Invariant fails to hold")
	}
	
	log.Println("TESTING INVARIANT 2 : Each client uses only 1 display name throughout its lifetime")
	clientValues := make(map[string]string)
	key := ""
	isCondTrue := true
	for _, fName := range s {
		f, err := os.Open(dirname + fName)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		foundVariable := false
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			s2 := scanner.Text()
			if foundVariable {
				foundVariable = false
				tempVals := strings.Split(s2, "\"")
				val := tempVals[1]
				if v2, ok := clientValues[key]; ok {
					if val != v2 {
						isCondTrue = isCondTrue && false
					}
				} else {
					clientValues[key] = val
				}
			} 
			if strings.Contains(s2, "display_name") {
				if !strings.HasPrefix(s2, "variable") {
					foundVariable = true
					tokens := strings.Split(s2, "-")
					key = tokens[0]
				}
			}
		}
	}

	if isCondTrue {
		log.Println("Invariant holds")
	} else {
		log.Println("Invariant fails to hold")
	}

	log.Println("TESTING INVARIANT 3 : The set of all display names of all clients is a subset of the list of display names the server knows of")
	var allValues map[string]bool
	for _, fName := range s {
		f, err := os.Open(dirname + fName)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		foundVariable := false
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			s2 := scanner.Text()
			if foundVariable {
				foundVariable = false
				allValues = parseValues(s2)
			} 
			if strings.Contains(s2, "displayNames") {
				if !strings.HasPrefix(s2, "variable") {
					foundVariable = true
				}
			}
		}
	}

	isSubset := true
	for _, v := range clientValues {
		if _, ok := allValues[v]; !ok {
			isSubset = isSubset && false
		}
	}

	if isSubset {
		log.Println("Invariant holds")
	} else {
		log.Println("Invariant fails to hold")
	}
}