package app

// EmptyAppOptions is a stub implementing servertypes.AppOptions.
// Lives in a non-test file so non-test consumers (e.g. testutil/network)
// can reference it.
type EmptyAppOptions struct{}

// Get implements AppOptions
func (ao EmptyAppOptions) Get(o string) interface{} {
	return nil
}
