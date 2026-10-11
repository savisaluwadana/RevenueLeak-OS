package pgstore

import "testing"

func TestWorkspaceIDMustBeUUID(t *testing.T) {
	for _, id := range []string{"", "alpha", "';DROP TABLE users;--", "00000000-0000-0000-0000-00000000000z"} {
		if s, err := Open("postgres://not-a-real-server", id); err == nil {
			s.Close()
			t.Errorf("unsafe workspace identifier accepted: %q", id)
		}
	}
}
