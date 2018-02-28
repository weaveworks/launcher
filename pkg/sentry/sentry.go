package sentry

import (
	raven "github.com/getsentry/raven-go"
)

// CaptureAndWait reports an error message to sentry along with a stacktrace.
//
// skipFrames can be used to indicate how many stack frames should be skipped to
// reach the frame where the error actually happened, eg.:
// - When Capture is called directly from the location that should appear in the
// stack trace, skipFrames should be 0.
// - When Capture is called from a helper function, eg. to both log to stdout
// and sentry, that helper function frame can be skipped by giving 1.
//
// tags are additional tags to add to the sentry event.
//
// This call is synchronous in that it will not return until the message has
// been sent to sentry.
func CaptureAndWait(skipFrames uint, message string, tags map[string]string) {
	// We want to skip the Capture() function itself.
	skipFrames++

	// Capture the error with stacktrace and wait.
	stack := raven.NewStacktrace(int(skipFrames), 3, nil)
	packet := raven.NewPacket(message, stack)
	eventID, ch := raven.Capture(packet, tags)
	if eventID != "" {
		<-ch
	}
}
