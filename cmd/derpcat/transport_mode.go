package main

import "os"

func usePublicDERPTransport() bool {
	return os.Getenv("DERPCAT_TEST_LOCAL_RELAY") != "1"
}
