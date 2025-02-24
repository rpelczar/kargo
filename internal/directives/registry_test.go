package directives

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirectivesRegistry_Register(t *testing.T) {
	t.Run("registers", func(t *testing.T) {
		registry := directivesRegistry{}
		promoter := &mockPromoter{}
		registry.register(promoter)
		assert.Same(t, promoter, registry[promoter.Name()])
	})

	t.Run("overwrites registration", func(t *testing.T) {
		registry := directivesRegistry{}
		promoter1 := &mockPromoter{}
		registry.register(promoter1)
		promoter2 := &mockPromoter{
			promoteErr: fmt.Errorf("error"),
		}
		registry.register(promoter2)
		assert.NotSame(t, promoter1, registry[promoter2.Name()])
		assert.Same(t, promoter2, registry[promoter2.Name()])
	})
}

func TestDirectivesRegistry_GetPromoter(t *testing.T) {
	t.Run("registration exists", func(t *testing.T) {
		registry := directivesRegistry{}
		orig := &mockPromoter{}
		registry.register(orig)
		promoter, err := registry.GetPromoter(orig.Name())
		assert.NoError(t, err)
		assert.Same(t, orig, promoter)
	})

	t.Run("registration does not exist", func(t *testing.T) {
		_, err := (directivesRegistry{}).GetPromoter("nonexistent")
		assert.ErrorContains(t, err, "not found")
	})
}
