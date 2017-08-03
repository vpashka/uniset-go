package uniset_test

import (
	"testing"
	"uniset"
)

func TestUniSet(t *testing.T) {

	var res = uniset.MyFirstFunc()

	if res != 42 {
		t.Error("Unknown result")
	}
}