package format

import (
	"fmt"
	"strings"
)

// FloatRU возвращает строку в формате "100.000.000,00"
func FloatRU(v float64, decimals int) string {
	s := fmt.Sprintf("%.*f", decimals, v) // "100000000.00"
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	var out []byte
	cnt := 0
	for i := len(intPart) - 1; i >= 0; i-- {
		out = append(out, intPart[i])
		cnt++
		if cnt%3 == 0 && i != 0 {
			out = append(out, '.')
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if decimals == 0 {
		return string(out)
	}
	return string(out) + "," + frac
}
