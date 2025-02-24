package directives

import (
	"testing"

	"github.com/stretchr/testify/assert"

	dirsdk "github.com/akuity/kargo/pkg/directives"
)

func TestState_Set(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() dirsdk.State
		key      string
		value    any
		expected any
	}{
		{
			name:     "Set string value",
			setup:    func() dirsdk.State { return make(dirsdk.State) },
			key:      "key1",
			value:    "value1",
			expected: "value1",
		},
		{
			name:     "Set integer value",
			setup:    func() dirsdk.State { return make(dirsdk.State) },
			key:      "key2",
			value:    42,
			expected: 42,
		},
		{
			name:     "Set slice value",
			setup:    func() dirsdk.State { return make(dirsdk.State) },
			key:      "key3",
			value:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Set map value",
			setup:    func() dirsdk.State { return make(dirsdk.State) },
			key:      "key4",
			value:    map[string]int{"a": 1, "b": 2},
			expected: map[string]int{"a": 1, "b": 2},
		},
		{
			name:     "Set nil value",
			setup:    func() dirsdk.State { return make(dirsdk.State) },
			key:      "key5",
			value:    nil,
			expected: nil,
		},
		{
			name: "Overwrite existing value",
			setup: func() dirsdk.State {
				s := make(dirsdk.State)
				s["key"] = "initial_value"
				return s
			},
			key:      "key",
			value:    "new_value",
			expected: "new_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setup()
			state.Set(tt.key, tt.value)
			assert.Equal(t, tt.expected, state[tt.key])
		})
	}
}

func TestState_Get(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() dirsdk.State
		key      string
		expected any
		exists   bool
	}{
		{
			name: "Get existing string value",
			setup: func() dirsdk.State {
				s := make(dirsdk.State)
				s["key1"] = "value1"
				return s
			},
			key:      "key1",
			expected: "value1",
			exists:   true,
		},
		{
			name: "Get existing integer value",
			setup: func() dirsdk.State {
				s := make(dirsdk.State)
				s["key2"] = 42
				return s
			},
			key:      "key2",
			expected: 42,
			exists:   true,
		},
		{
			name: "Get existing slice value",
			setup: func() dirsdk.State {
				s := make(dirsdk.State)
				s["key3"] = []string{"a", "b", "c"}
				return s
			},
			key:      "key3",
			expected: []string{"a", "b", "c"},
			exists:   true,
		},
		{
			name: "Get existing map value",
			setup: func() dirsdk.State {
				s := make(dirsdk.State)
				s["key4"] = map[string]int{"a": 1, "b": 2}
				return s
			},
			key:      "key4",
			expected: map[string]int{"a": 1, "b": 2},
			exists:   true,
		},
		{
			name: "Get existing nil value",
			setup: func() dirsdk.State {
				s := make(dirsdk.State)
				s["key5"] = nil
				return s
			},
			key:      "key5",
			expected: nil,
			exists:   true,
		},
		{
			name: "Get non-existent key",
			setup: func() dirsdk.State {
				return make(dirsdk.State)
			},
			key:      "non_existent",
			expected: nil,
			exists:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setup()
			value, ok := state.Get(tt.key)

			assert.Equal(t, tt.expected, value)
			assert.Equal(t, tt.exists, ok)
		})
	}
}
