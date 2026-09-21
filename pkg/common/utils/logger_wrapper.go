// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package utils

import (
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

type logrusWrapper struct {
	log   *logrus.Logger
	entry *logrus.Entry
}

// Init implements logr.LogSink
func (l *logrusWrapper) Init(info logr.RuntimeInfo) {
}

func (l *logrusWrapper) Enabled(level int) bool {
	return true
}

func (l *logrusWrapper) Info(level int, msg string, keysAndValues ...interface{}) {
	l.getEntry().WithFields(l.parseFields(keysAndValues)).Info(msg)
}

func (l *logrusWrapper) Error(err error, msg string, keysAndValues ...interface{}) {
	l.getEntry().WithError(err).WithFields(l.parseFields(keysAndValues)).Error(msg)
}

func (l *logrusWrapper) V(level int) logr.LogSink {
	return l
}

func (l *logrusWrapper) WithValues(keysAndValues ...interface{}) logr.LogSink {
	entry := l.getEntry().WithFields(l.parseFields(keysAndValues))
	return &logrusWrapper{log: l.log, entry: entry}
}

func (l *logrusWrapper) parseFields(keysAndValues []interface{}) logrus.Fields {
	res := logrus.Fields{}
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if ok {
			res[key] = keysAndValues[i+1]
		}
	}
	return res
}

func (l *logrusWrapper) getEntry() *logrus.Entry {
	if l.entry != nil {
		return l.entry
	}
	logger := l.log
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return logrus.NewEntry(logger)
}

func (l *logrusWrapper) WithName(name string) logr.LogSink {
	if existing, ok := l.getEntry().Data["name"].(string); ok && existing != "" {
		name = existing + "/" + name
	}
	entry := l.getEntry().WithField("name", name)
	return &logrusWrapper{log: l.log, entry: entry}
}

func NewLogger() *logrus.Logger {
	log := logrus.New()
	log.SetReportCaller(true)
	log.SetFormatter(&logrus.JSONFormatter{})
	return log
}

func NewLogWrapper() *logrusWrapper {
	return &logrusWrapper{
		log: NewLogger(),
	}
}
