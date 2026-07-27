package config

import "os"

const defaultServerAddress = "127.0.0.1:19080"

func ServerAddress() string {
	if address := os.Getenv("TEST_AUTO_PRO_SERVER_ADDR"); address != "" {
		return address
	}
	return defaultServerAddress
}
