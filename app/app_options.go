package app

// EmptyAppOptions is a stub implementing servertypes.AppOptions.
type EmptyAppOptions struct{}

// Get implements AppOptions
func (ao EmptyAppOptions) Get(o string) interface{} {
	return nil
}
