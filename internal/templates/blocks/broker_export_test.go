package blocks

import "github.com/nathanhollows/Rapua/v8/blocks"

// Export internal functions for testing purposes.
func GetBrokerInfoReceived(state blocks.PlayerState) string {
	return getBrokerInfoReceived(state)
}
