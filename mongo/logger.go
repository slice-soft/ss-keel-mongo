package mongo

// Logger is the logging interface expected by this package.
// It is satisfied by *logger.Logger from ss-keel-core.
type Logger interface {
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
}
