package messageidentity

import (
	"fmt"
	"strconv"
	"strings"
)

const factVersionMarker = ":v"

// Fact returns a stable identity for one immutable aggregate fact. The
// aggregate version distinguishes recurring events of the same type while a
// retry of the same fact produces exactly the same identity.
func Fact(aggregateID, step string, aggregateVersion int64) string {
	return fmt.Sprintf("%s:%s%s%d", aggregateID, step, factVersionMarker, aggregateVersion)
}

// FactVersion extracts an aggregate version from a Fact identity. Command IDs
// deliberately do not carry this suffix and return false.
func FactVersion(messageID string) (int64, bool) {
	marker := strings.LastIndex(messageID, factVersionMarker)
	if marker < 0 || marker+len(factVersionMarker) == len(messageID) {
		return 0, false
	}
	version, err := strconv.ParseInt(messageID[marker+len(factVersionMarker):], 10, 64)
	if err != nil || version <= 0 {
		return 0, false
	}
	return version, true
}
