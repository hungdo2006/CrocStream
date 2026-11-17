//go:build g4test

package desktop

// Test builds only: exercises the actual receive-loop PLI recovery path.
func (s *Service) InvalidateViewerReference() { s.stream.RequestRecovery() }
