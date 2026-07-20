//go:build darwin

package bluetooth

func preferredTransports() []Transport {
	return []Transport{
		&BLETransport{},
	}
}

func CheckDependencies() []DependencyStatus {
	return []DependencyStatus{}
}
