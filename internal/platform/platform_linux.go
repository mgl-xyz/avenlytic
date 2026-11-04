//go:build linux

package platform

import (
	ports "github.com/avenlytic/avenlytic/internal/core/ports"
	"github.com/avenlytic/avenlytic/internal/platform/linux"
)

func Current() ports.Provider { return linux.New() }
