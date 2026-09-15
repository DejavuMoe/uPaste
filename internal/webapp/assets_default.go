//go:build !production

package webapp

// NewEmbedded returns no handler because this build has no embedded frontend.
// Development builds serve the UI from the Vite development server.
func NewEmbedded() (*Handler, error) { return nil, nil }
