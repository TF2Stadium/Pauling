package main

import (
	"os"
)

func overrideFromEnv(constant *string, name string) {
	val := os.Getenv(name)
	if "" != val {
		*constant = val
	}
}

func overrideBoolFromEnv(constant *bool, name string) {
	val := os.Getenv(name)
	if val != "" {
		*constant = map[string]bool{
			"true":  true,
			"false": false,
		}[val]
	}
}
