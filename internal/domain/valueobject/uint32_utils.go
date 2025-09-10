package valueobject

// MaxUint32 represents the maximum value of a uint32.
const MaxUint32 = ^uint32(0)

// ClampToUint32 safely converts an int to uint32 by clamping
// negative values to 0 and values larger than MaxUint32 to MaxUint32.
func ClampToUint32(i int) uint32 {
	if i <= 0 {
		return 0
	}
	if i > int(MaxUint32) {
		return MaxUint32
	}
	return uint32(i)
}

// AddUint32Clamped returns a + b, clamped at MaxUint32 on overflow.
func AddUint32Clamped(a, b uint32) uint32 {
	if a > MaxUint32-b {
		return MaxUint32
	}
	return a + b
}

// ClampUintToUint32 safely converts a uint to uint32 by clamping to MaxUint32 when needed.
func ClampUintToUint32(u uint) uint32 {
	if u > uint(MaxUint32) {
		return MaxUint32
	}
	return uint32(u)
}
