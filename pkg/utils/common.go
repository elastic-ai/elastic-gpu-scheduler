package utils

func CloneInts(array []int) []int {
	ans := make([]int, len(array), cap(array))
	copy(ans, array)
	return ans
}
