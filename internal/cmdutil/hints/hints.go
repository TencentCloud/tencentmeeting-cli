package hints

// HintProvider defines the interface for hints providers.
// Commands that need to return hints should implement this interface,
// and the corresponding method will be invoked in WithHints.
type HintProvider interface {
	// Hints returns the hints messages.
	Hints(string) []string
}
