// Package all registers every format implemented by opentile-go. Import this
// package for its side effect (via a blank import) or call Register() once
// from main for equivalent behavior without relying on import ordering.
//
//	import _ "github.com/tcornish/opentile-go/formats/all"
//
// Or:
//
//	import formats_all "github.com/tcornish/opentile-go/formats/all"
//	...
//	formats_all.Register()
package all

import (
	"sync"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/formats/ndpi"
	"github.com/tcornish/opentile-go/formats/svs"
)

var once sync.Once

// Register registers all known format factories with the top-level opentile
// package. Safe to call multiple times; only the first call registers.
func Register() {
	once.Do(func() {
		opentile.Register(svs.New())
		opentile.Register(ndpi.New())
	})
}

func init() { Register() }
