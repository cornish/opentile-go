package ndpi

import (
	"math"

	opentile "github.com/cornish/opentile-go"
)

// AdjustTileSize returns the output tile size to use for an NDPI tiler given
// the user's requested size and the smallest native stripe width in the file.
//
// Upstream opentile's algorithm: the adjusted size is a power-of-2 multiple
// of the smallest stripe width, where the exponent is
// round(log2(ratio(requested, stripe))). If there are no striped pages
// (stripeWidth == 0), the request passes through unchanged. The result is
// always square.
//
// Concretely this guarantees every output tile is an integer number of
// native stripes wide, so the stripe-concat code never needs to crop
// horizontally within a stripe — it just concatenates whole stripes.
func AdjustTileSize(requested, stripeWidth int) opentile.Size {
	if stripeWidth == 0 || requested == stripeWidth {
		return opentile.Size{W: requested, H: requested}
	}
	var factor float64
	if requested > stripeWidth {
		factor = float64(requested) / float64(stripeWidth)
	} else {
		factor = float64(stripeWidth) / float64(requested)
	}
	factor2 := math.Pow(2, math.Round(math.Log2(factor)))
	adjusted := int(factor2) * stripeWidth
	return opentile.Size{W: adjusted, H: adjusted}
}
