package rustrt

import "math"

// Pow reproduces reference f64 exponentiation with compensated log2/exp2 evaluation.
// Explicit fused operations and intermediate rounding preserve its final-bit behavior.
func Pow(x, y float64) float64 {
	if y == 0 || x == 1 {
		return 1
	}
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x == 0 || y == 1 {
		return math.Pow(x, y)
	}
	integer := math.Trunc(y) == y
	if x < 0 && !integer {
		return math.NaN()
	}
	sign := uint64(0)
	if x < 0 && math.Abs(y) < 0x1p53 && int64(y)&1 != 0 {
		sign = 1 << 63
	}
	x = math.Abs(x)
	if math.Abs(y) > 0x1p64 || math.Abs(y) < 0x1p-65 {
		return math.Pow(math.Copysign(x, math.Float64frombits(sign)), y)
	}
	bits := math.Float64bits(x)
	if bits < 0x0010000000000000 {
		normalized := math.Float64frombits(bits|0x3ff0000000000000) - 1
		bits = math.Float64bits(normalized) + 0xc020000000000000
	}
	fraction := bits & 0x000fffffffffffff
	var high, low float64
	if fraction == 0 {
		logarithm := math.Ldexp(float64(int64(bits-0x3ff0000000000000)), -45)
		high = float64(logarithm * y)
		low = math.FMA(logarithm, y, -high)
	} else {
		index := ((fraction + 0x0000100000000000) & 0x001fe00000000000) >> 45
		entry := powLogTable[index]
		scale := entry[0]
		m := math.Float64frombits(fraction | 0x3ff0000000000000)
		product := float64(scale * m)
		mHigh := math.Float64frombits(math.Float64bits(m) & 0xffffffffffe00000)
		mLow := m - mHigh
		residual := math.FMA(scale, mHigh, -product)
		z := product - 1
		zLow := math.FMA(scale, mLow, residual)
		c := powLogCoefficients
		tail := math.FMA(z, c[0], c[1])
		tail = math.FMA(z, tail, c[2])
		tail = math.FMA(z, tail, c[3])
		tail = math.FMA(z, tail, c[4])
		z2 := float64(z * z)
		z4 := float64(z2 * z2)
		tail = float64(z4 * tail)
		qHigh := math.FMA(c[5], z, c[7])
		qLow := math.FMA(c[6], z, c[8])
		residual = qHigh - c[7]
		residual = math.FMA(c[5], z, -residual)
		qLow = math.FMA(c[5], zLow, qLow)
		qLow = residual + qLow
		pHigh := math.FMA(qHigh, z, c[9])
		pLow := math.FMA(qLow, z, c[10])
		residual = pHigh - c[9]
		residual = math.FMA(qHigh, z, -residual)
		pLow = math.FMA(qHigh, zLow, pLow)
		pLow = residual + pLow
		main := float64(pHigh * z)
		pLow = float64(pLow * z)
		residual = math.FMA(pHigh, z, -main)
		pLow = math.FMA(pHigh, zLow, pLow)
		residual = residual + pLow
		exponent := float64(int64(bits+0xc018100000000000) >> 52)
		logHigh := exponent + entry[1]
		logHigh = logHigh + main
		logHigh = logHigh + tail
		logHigh = math.Float64frombits(math.Float64bits(logHigh) & 0xfffffffffffffffe)
		errorPart := exponent - logHigh
		errorPart = errorPart + entry[1]
		errorPart = errorPart + main
		errorPart = errorPart + tail
		logLow := entry[2] + errorPart
		logLow = logLow + residual
		yScaled := float64(y * 128)
		high = float64(yScaled * logHigh)
		low = math.FMA(yScaled, logHigh, -high)
		low = math.FMA(yScaled, logLow, low)
	}
	if high > 0x1.004p17 {
		return math.Float64frombits(sign | 0x7ff0000000000000)
	}
	if high < -0x1.0ccp17 {
		return math.Float64frombits(sign)
	}
	floor := math.Floor(high)
	n := int64(floor)
	f := high - floor
	entry := powExpTable[uint64(n)&127]
	reduced := low - entry[1]
	reduced = reduced + f
	c := powExpCoefficients
	poly := math.FMA(f, c[0], c[1])
	poly = math.FMA(f, poly, c[2])
	poly = math.FMA(reduced, poly, c[3])
	poly = math.FMA(reduced, poly, c[4])
	poly = float64(reduced * poly)
	first := (uint64(n) << 44) & 0xfff0000000000000
	second := (uint64(n) << 45) & 0xfff0000000000000
	second = second - first
	first = first ^ sign
	result := math.Float64frombits(first + math.Float64bits(entry[0]))
	result = math.FMA(result, poly, result)
	scale := math.Float64frombits(second + 0x3ff0000000000000)
	return float64(result * scale)
}
