//go:build !linux && !windows && !darwin

package platform

import ports "github.com/avenlytic/avenlytic/internal/core/ports"

func Current() ports.Provider { return genericProvider{} }
