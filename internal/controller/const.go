package controller

import "time"

const (
	requeueTime     = 5 * time.Second
	domainName      = ".didactiklabs.io"
	unsealFinalizer = "unseal" + domainName
)
