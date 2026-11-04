//go:build windows

package platform

import (
	ports "github.com/avenlytic/avenlytic/internal/core/ports"
	"github.com/avenlytic/avenlytic/internal/platform/windows"
)

func Current() ports.Provider { return windows.New() }
