package domain

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

// The room picker owns the list of interview rooms (KAN-275). The backend copy in Environments must equal
// it, so a room added or renamed there fails here until the backend is updated too.
func TestEnvironmentsMatchTheFrontendRoomPicker(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "frontend", "src", "features", "interview", "components", "room-backgrounds.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the frontend room picker is the source of the environment ids: %v", err)
	}
	// The picker lists its rooms as scene("cafe-3d", ...) and photo("office"); the function definitions
	// take parameters, not string literals, so only real entries match.
	var found []string
	for _, match := range regexp.MustCompile(`\b(?:scene|photo)\("([a-z0-9-]+)"`).FindAllStringSubmatch(string(source), -1) {
		found = append(found, match[1])
	}
	want := append([]string(nil), Environments...)
	sort.Strings(found)
	sort.Strings(want)
	if len(found) == 0 || !reflect.DeepEqual(found, want) {
		t.Fatalf("room-backgrounds.ts lists %v, domain.Environments is %v", found, want)
	}
}
