package controller

import "time"

const (
	requeueTime     = 60 * time.Second
	domainName      = ".didactiklabs.io"
	unsealFinalizer = "unseal" + domainName
)
