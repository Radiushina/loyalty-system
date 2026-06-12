package luhn

import "errors"

var (
	ErrInvalidOrderNumber = errors.New("invalid order number format")
)

func Valid(number string) bool {
	sum := 0
	parity := len(number) % 2

	for i := 0; i < len(number); i++ {
		if number[i] < '0' || number[i] > '9' {
			return false
		}

		d := int(number[i] - '0')
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}

	return sum%10 == 0
}
