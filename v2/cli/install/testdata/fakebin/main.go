// fakebin stands in for the mindfries binary in the installer tests.
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() { fmt.Println("fakebin " + strings.Join(os.Args[1:], " ")) }
