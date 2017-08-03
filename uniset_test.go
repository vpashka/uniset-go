package uniset_test

import (
//"uniset"
"testing"
	"os"
	"strings"
	"fmt"
)

func TestUniSet(t *testing.T) {
	who := "World!"

	if len (os.Args) > 1 {  /* os.Args[0] - имя команды «hello» или «hello.exe» */

		who = strings.Join(os.Args[1:], " ")

	}
	fmt.Println("Hello", who)

}