//+build gofuzz

package example

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/leastauthority/lafuzz/fuzzing"
)

var crashLimit int

func TestMain(m *testing.M) {
	flag.IntVar(&crashLimit, "crash-limit", 1000, "number of crashing inputs to test before stopping")
	flag.Parse()
	os.Exit(m.Run())
}

func TestFuzzBuggyFunc(t *testing.T) {
	_, panics, _ := fuzzing.
		MustNewCrasherIterator(FuzzBuggyFunc).
		TestFailingLimit(t, crashLimit)

	require.Zero(t, panics)
}
