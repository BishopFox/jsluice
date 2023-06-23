package main

import (
	"fmt"
	"strings"

	"github.com/BishopFox/jsluice"
)

func printTree(opts options, filename string, source []byte, output chan string, errs chan error) {

	buf := strings.Builder{}
	buf.WriteString(fmt.Sprintf("%s:\n", filename))

	buf.WriteString(jsluice.PrintTree(source))

	output <- buf.String()
}
