package testutil

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/envadiv/Passage3D/testutil/network"
)

func TestIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in-process network test in short mode")
	}
	cfg := network.DefaultConfig()
	// One validator: with several, the first block waits for the nodes to
	// dial each other, which can exceed the SDK network's start-up timeout.
	cfg.NumValidators = 1
	suite.Run(t, NewIntegrationTestSuite(cfg))
}
