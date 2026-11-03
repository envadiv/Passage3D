package iavl

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbm "github.com/cosmos/iavl/db"
)

// nilEmptyValueDB reproduces what was observed on a Passage node using the
// default goleveldb backend: for some records, a key stored with a zero-length
// value is returned as nil by Get, so Has reports it as missing, while an
// iterator still finds the key.
type nilEmptyValueDB struct {
	dbm.DB
}

func (d nilEmptyValueDB) Get(key []byte) ([]byte, error) {
	v, err := d.DB.Get(key)
	if len(v) == 0 {
		return nil, err
	}
	return v, err
}

func (d nilEmptyValueDB) Has(key []byte) (bool, error) {
	v, err := d.Get(key)
	return v != nil, err
}

// TestPassageEmptyTreeVersions covers the Passage empty-store fixes in nodedb.go
// (hasVersion / hasLegacyVersion / GetRoot). An empty tree stores its root with a
// zero-length value. With the backend behaviour above, the pre-v4.0.4 hasVersion
// (db.Has) made getFirstVersion report only the latest version of an empty store:
// `passage rollback` failed with "version does not exist" (verified end-to-end on
// a node: x/evidence, x/feegrant, x/authz, x/circuit, x/claim stores), and
// historical queries / `export --height` failed as well.
func TestPassageEmptyTreeVersions(t *testing.T) {
	const versions = 30
	db := nilEmptyValueDB{dbm.NewMemDB()}

	tree := NewMutableTree(db, 0, false, NewNopLogger())
	for i := 1; i <= versions; i++ {
		_, v, err := tree.SaveVersion()
		require.NoError(t, err)
		require.Equal(t, int64(i), v)
	}

	// A fresh tree on the same DB recomputes first/latest versions from disk,
	// as a restarted node does.
	reopened := NewMutableTree(db, 0, false, NewNopLogger())
	latest, err := reopened.Load()
	require.NoError(t, err)
	require.Equal(t, int64(versions), latest)

	for v := int64(1); v <= versions; v++ {
		require.Truef(t, reopened.VersionExists(v), "version %d of an empty tree must exist", v)
	}
	require.Len(t, reopened.AvailableVersions(), versions)

	// Historical read of an empty tree.
	imm, err := reopened.GetImmutable(versions / 2)
	require.NoError(t, err)
	require.Equal(t, int64(0), imm.Size())

	// What `passage rollback` does for every store.
	rollback := NewMutableTree(db, 0, false, NewNopLogger())
	require.NoError(t, rollback.LoadVersionForOverwriting(versions-1))
	require.Equal(t, int64(versions-1), rollback.Version())
}
