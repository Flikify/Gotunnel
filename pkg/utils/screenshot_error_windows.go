//go:build windows

package utils

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

const windowsNoActiveConsoleSession = 0xFFFFFFFF

func annotateScreenshotError(err error) error {
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "BitBlt failed") {
		return err
	}

	sessionID, sessionErr := currentProcessSessionID()
	if sessionErr == nil {
		if sessionID == 0 {
			return fmt.Errorf("%w (client is running as a Windows service in session 0, so GDI cannot capture the interactive desktop; run the client in the logged-in user session instead)", err)
		}

		activeSessionID := windows.WTSGetActiveConsoleSessionId()
		if activeSessionID != windowsNoActiveConsoleSession && sessionID != activeSessionID {
			return fmt.Errorf("%w (client is running in Windows session %d, but the active console session is %d; screenshot capture only works from the interactive user session)", err, sessionID, activeSessionID)
		}
	}

	return fmt.Errorf("%w (the Windows desktop may be locked, switched to the secure desktop by UAC, or otherwise unavailable to GDI screen capture)", err)
}

func currentProcessSessionID() (uint32, error) {
	var sessionID uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &sessionID); err != nil {
		return 0, err
	}
	return sessionID, nil
}
