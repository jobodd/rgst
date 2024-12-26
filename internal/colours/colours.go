package colours

import "strconv"

const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Reset  = "\033[0m"
)

func ColouredInt(i int, colour string) string {
	return ColouredString(strconv.Itoa(i), colour)
}
func ColouredString(s string, colour string) string {
	return colour + s + Reset
}
