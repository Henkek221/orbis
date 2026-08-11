//go:build !darwin

package config

func discoverSystemTokenP1() string {
	return ""
}
