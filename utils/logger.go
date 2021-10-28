package utils

import (
	"encoding/json"
	"fmt"
)

// Logger is used to produce one structured JSON log message for each Lambda
// function invocation.
type Logger map[string]string

// Clear empties anything in the Log, and should be called at the beginning of
// each Lambda function invocation.
func (l Logger) Clear() {
	for key := range l {
		delete(l, key)
	}
}

// Set provides a key-value pair to present in the final printed log.
func (l Logger) Set(key string, val string) {
	l[key] = val
}

// Print writes the log as JSON to stdout, and should be called in a deferred
// function on each Lambda function invocation.
func (l Logger) Print() {
	if len(l) == 0 {
		return
	}

	if data, err := json.Marshal(l); err == nil {
		fmt.Println(string(data))
	}
}
