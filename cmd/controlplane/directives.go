package main

import (
	intdirs "github.com/akuity/kargo/internal/directives"
	"github.com/akuity/kargo/pkg/directives"
)

func RegisterDirective(directive directives.Directive) {
	intdirs.Register(directive)
}
