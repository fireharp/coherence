package main

// GT: live
func sharedUtil(x int) int {
	return x * 2
}

// GT: dead-code
func unusedUtil(s string) string {
	return s + "!"
}
