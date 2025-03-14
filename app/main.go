package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"unicode"

	bencode "github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	if len(bencodedString) == 0 {
		return nil, fmt.Errorf("bencoded string is empty")
	}

	if unicode.IsDigit(rune(bencodedString[0])) {
		decodedString, err := bencode.DecodeBencodedString(bencodedString)

		if err != nil {
			return "", err
		}

		return decodedString, nil
	} else if bencodedString[0] == 'i' {
		decodedInterger, err := bencode.DecodeBencodedInteger(bencodedString)

		if err != nil {
			return 0, err
		}

		return decodedInterger, nil
	} else if bencodedString[0] == 'l' {
		decodedList, err := bencode.DecodeBencodedList(bencodedString)

		if err != nil {
			return nil, err
		}

		return decodedList, nil
	} else {
		return "", fmt.Errorf("Only strings are supported at the moment")
	}
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	command := os.Args[1]

	if command == "decode" {
		// Uncomment this block to pass the first stage
		//
		bencodedValue := os.Args[2]
		decoded, err := decodeBencode(bencodedValue)

		if err != nil {
			log.Fatal(err)
			return
		}

		if unicode.IsDigit(rune(bencodedValue[0])) {
			fmt.Printf("\"%s\"\n", decoded)
			return
		}

		fmt.Printf("%v\n", decoded)
		return
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
