//go:build !darwin && !linux

package trust

import (
	"fmt"
	"runtime"
)

type unsupportedTrustor struct{}

func newPlatformTrustor() Trustor {
	return &unsupportedTrustor{}
}

func (u *unsupportedTrustor) Install(rootCertPEM []byte) error {
	return fmt.Errorf("trust: automatic trust store management is not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func (u *unsupportedTrustor) Uninstall() error {
	return fmt.Errorf("trust: automatic trust store management is not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func (u *unsupportedTrustor) IsInstalled(rootCertPEM []byte) bool {
	return false
}

func (u *unsupportedTrustor) NeedsElevation() bool {
	return false
}
