package main

import "fmt"

func main() {
	// GT: live
	fmt.Println(activeHandler())
	registerExportedAPI()
}

// GT: live
func activeHandler() string {
	return helperA() + helperB()
}

// GT: live
func helperA() string {
	return "a"
}

// GT: live
func helperB() string {
	return "b" + nestedHelper()
}

// GT: live
func nestedHelper() string {
	return "n"
}

// GT: dead-code
func orphanedInternal() string {
	return "never called"
}

// GT: dead-code
func anotherOrphan() int {
	return 42
}

// GT: exported-do-not-flag — exported, no caller in this corpus, but library API
func RegisterExportedAPI() string {
	return helperA()
}

// GT: live (indirect)
func registerExportedAPI() {
	_ = RegisterExportedAPI()
}
