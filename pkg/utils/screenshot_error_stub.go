//go:build !windows

package utils

func annotateScreenshotError(err error) error {
	return err
}
