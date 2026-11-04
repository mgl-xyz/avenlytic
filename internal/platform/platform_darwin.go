//go:build darwin

package platform

import (
	ports "github.com/avenlytic/avenlytic/internal/core/ports"
	"github.com/avenlytic/avenlytic/internal/platform/darwin"
)

func Current() ports.Provider { return darwin.New() }
