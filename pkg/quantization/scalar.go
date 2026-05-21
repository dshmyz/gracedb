package quantization

import "math"

// ScalarQuantize performs scalar (uniform) quantization on a vector.
// Each float32 component is quantized to the specified number of bits.
func ScalarQuantize(vector []float32, bits int) ([]byte, []float32, []float32) {
	if bits != 8 {
		bits = 8
	}

	// Find min/max.
	minVal := float32(math.MaxFloat32)
	maxVal := float32(-math.MaxFloat32)
	for _, v := range vector {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	if minVal == maxVal {
		minVal = 0
		maxVal = 1
	}

	// Quantize to 8-bit.
	rangeVal := maxVal - minVal
	if rangeVal == 0 {
		rangeVal = 1
	}
	scale := 255.0 / float64(rangeVal)
	quantized := make([]byte, len(vector))
	for i, v := range vector {
		q := (float64(v) - float64(minVal)) * scale
		if q < 0 {
			q = 0
		}
		if q > 255 {
			q = 255
		}
		quantized[i] = byte(q)
	}

	return quantized, []float32{minVal}, []float32{maxVal}
}

// ScalarDequantize reconstructs a scalar-quantized vector.
func ScalarDequantize(quantized []byte, minVal, maxVal float32) []float32 {
	scale := (maxVal - minVal) / 255
	dequantized := make([]float32, len(quantized))
	for i, q := range quantized {
		dequantized[i] = minVal + float32(q)*scale
	}
	return dequantized
}

// BinaryQuantize binarizes a vector to ±1 based on sign.
func BinaryQuantize(vector []float32) []byte {
	result := make([]byte, (len(vector)+7)/8)
	for i, v := range vector {
		if v >= 0 {
			result[i/8] |= (1 << (i % 8))
		}
	}
	return result
}

// BinarySimilarity computes Jaccard-like similarity between binary vectors.
func BinarySimilarity(a, b []byte, dim int) float32 {
	matches := 0
	total := 0
	for i := 0; i < len(a) && i < len(b); i++ {
		x := a[i] ^ b[i] // XOR: bits that differ
		for j := 0; j < 8; j++ {
			if total < dim {
				if (x & (1 << j)) == 0 {
					matches++
				}
				total++
			}
		}
	}
	if total == 0 {
		return 1.0
	}
	return float32(matches) / float32(total)
}
